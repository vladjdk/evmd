# Converting a Cosmos SDK Chain to an EVM Chain: Step-by-Step Guide

This guide provides detailed steps to convert a standard Cosmos SDK chain into an EVM-compatible chain. Follow these instructions carefully to add Ethereum Virtual Machine functionality to your existing Cosmos SDK blockchain.

## Prerequisites

- A working Cosmos SDK chain on v0.50.x
- IBC-Go v8
- Go 1.23+ installed
- Basic knowledge of Go and Cosmos SDK

## Step 1: Update Dependencies in go.mod

```
// import modules
require (
    github.com/cosmos/cosmos-sdk v0.50.13
	github.com/ethereum/go-ethereum v1.10.26
	
	// for ibc functionality in EVM
    github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8 v8.1.1
	github.com/cosmos/ibc-go/modules/capability v1.0.1
	github.com/cosmos/ibc-go/v8 v8.7.0
)
```

```
// Add module replacements
replace (
    cosmossdk.io/store => github.com/cosmos/cosmos-sdk/store v1.1.2-0.20250319183239-53dea340efc7
    github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.50.13-0.20250319183239-53dea340efc7
    github.com/ethereum/go-ethereum => github.com/cosmos/go-ethereum v1.10.26-evmos-rc4
)
```

## Step 2: Update Chain Configuration Settings

### Update Chain ID Format
Modify your chain ID to use EVM-compatible format. Example: Change from "localchain-1" to "localchain_9000-1".
```bash
# 
# For example, update in the following files (if your project has them):
# - Makefile
# - app/app.go 
# - chains/standalone.json
# - chains/self-ibc.json
# - chains/testnet.json
# - scripts/test_node.sh
```

### Update Coin Type
Change the coin type value from 118 (Cosmos) to 60 (Ethereum) if your chain has that explicitly defined:
```bash
# In app/app.go:
# CoinType uint32 = 60

# In chain_registry.json:
# "slip44": 60,

# In test files & chain configs:
# "coin_type": "60",
```

### Update BaseDenomUnit
Change base denom unit from 6 (Cosmos) to 18 (EVM):
```bash
# In app/app.go:
# BaseDenomUnit int64 = 18

# In chain_registry_assets.json:
# "exponent": 18
```

### Add SDK Power Reduction Update
In app/app.go, add the following init function:
```go
func init() {
	// manually update the power reduction based on the base denom unit (10^18 [evm] or 10^6 [cosmos]) 
	//DefaultPowerReduction is the default amount of staking tokens required for 1 unit of consensus-engine power
    sdk.DefaultPowerReduction = math.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))
}
```

## Step 3: Create EVM Configuration File

Create a new file `app/config.go` with the following content:
```go
package app

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// EVMOptionsFn defines a function type for setting app options specifically for
// the app. The function should receive the chainID and return an error if
// any.
type EVMOptionsFn func(string) error

// NoOpEVMOptions is a no-op function that can be used when the app does not
// need any specific configuration.
func NoOpEVMOptions(_ string) error {
	return nil
}

var sealed = false

// ChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var ChainsCoinInfo = map[string]evmtypes.EvmCoinInfo{
	ChainID: {
		Denom:        BaseDenom,
		DisplayDenom: DisplayDenom,
		Decimals:     evmtypes.EighteenDecimals,
	},
}

// EVMAppOptions allows to setup the global configuration
// for the chain.
func EVMAppOptions(chainID string) error {
	if sealed {
		return nil
	}

	if chainID == "" {
		chainID = ChainID
	}

	id := strings.Split(chainID, "-")[0]
	coinInfo, found := ChainsCoinInfo[id]
	if !found {
		coinInfo, found = ChainsCoinInfo[chainID]
		if !found {
			return fmt.Errorf("unknown chain id: %s, %+v", chainID, ChainsCoinInfo)
		}
	}

	// set the denom info for the chain
	if err := setBaseDenom(coinInfo); err != nil {
		return err
	}

	baseDenom, err := sdk.GetBaseDenom()
	if err != nil {
		return err
	}

	ethCfg := evmtypes.DefaultChainConfig(chainID)

	err = evmtypes.NewEVMConfigurator().
		WithChainConfig(ethCfg).
		// NOTE: we're using the 18 decimals
		WithEVMCoinInfo(baseDenom, uint8(coinInfo.Decimals)).
		Configure()
	if err != nil {
		return err
	}

	sealed = true
	return nil
}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain.
func setBaseDenom(ci evmtypes.EvmCoinInfo) error {
	if err := sdk.RegisterDenom(ci.DisplayDenom, math.LegacyOneDec()); err != nil {
		return err
	}

	// sdk.RegisterDenom will automatically overwrite the base denom when the
	// new setBaseDenom() are lower than the current base denom's units.
	return sdk.RegisterDenom(ci.Denom, math.LegacyNewDecWithPrec(1, int64(ci.Decimals)))
}
```

## Step 4: Create Token Pair Configuration TODO: move down to testing

Create a new file `app/token_pair.go` with the following content:
```go
package app

import erc20types "github.com/cosmos/evm/x/erc20/types"

// WTokenContractMainnet is the WrappedToken contract address for mainnet
const WTokenContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"

// ExampleTokenPairs creates a slice of token pairs, that contains a pair for the native denom of the example chain
// implementation.
var ExampleTokenPairs = []erc20types.TokenPair{
    {
        Erc20Address:  WTokenContractMainnet,
        Denom:         BaseDenom,
        Enabled:       true,
        ContractOwner: erc20types.OWNER_MODULE,
    },
}
```

## Step 5: Create Precompiles Configuration

Create a file `app/precompiles.go`:
```go
package app

import (
	"fmt"
	"maps"

	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bech32"
	distprecompile "github.com/cosmos/evm/precompiles/distribution"
	evidenceprecompile "github.com/cosmos/evm/precompiles/evidence"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	ics20precompile "github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/p256"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	"github.com/cosmos/evm/x/vm/core/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v8/modules/core/04-channel/keeper"
	"github.com/ethereum/go-ethereum/common"
)

const bech32PrecompileBaseGas = 6_000

// NewAvailableStaticPrecompiles returns the list of all available static precompiled contracts from EVM.
//
// NOTE: this should only be used during initialization of the Keeper.
func NewAvailableStaticPrecompiles(
	stakingKeeper stakingkeeper.Keeper,
	distributionKeeper distributionkeeper.Keeper,
	bankKeeper bankkeeper.Keeper,
	erc20Keeper erc20Keeper.Keeper,
	authzKeeper authzkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
	channelKeeper channelkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	govKeeper govkeeper.Keeper,
	slashingKeeper slashingkeeper.Keeper,
	evidenceKeeper evidencekeeper.Keeper,
) map[common.Address]vm.PrecompiledContract {
	// Clone the mapping from the latest EVM fork.
	precompiles := maps.Clone(vm.PrecompiledContractsBerlin)

	// secp256r1 precompile as per EIP-7212
	p256Precompile := &p256.Precompile{}

	bech32Precompile, err := bech32.NewPrecompile(bech32PrecompileBaseGas)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}

	stakingPrecompile, err := stakingprecompile.NewPrecompile(stakingKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate staking precompile: %w", err))
	}

	distributionPrecompile, err := distprecompile.NewPrecompile(
		distributionKeeper,
		stakingKeeper,
		authzKeeper,
		evmKeeper,
	)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate distribution precompile: %w", err))
	}

	ibcTransferPrecompile, err := ics20precompile.NewPrecompile(
		stakingKeeper,
		transferKeeper,
		channelKeeper,
		authzKeeper,
		evmKeeper,
	)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate ICS20 precompile: %w", err))
	}

	bankPrecompile, err := bankprecompile.NewPrecompile(bankKeeper, erc20Keeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bank precompile: %w", err))
	}

	govPrecompile, err := govprecompile.NewPrecompile(govKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate gov precompile: %w", err))
	}

	slashingPrecompile, err := slashingprecompile.NewPrecompile(slashingKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate slashing precompile: %w", err))
	}

	evidencePrecompile, err := evidenceprecompile.NewPrecompile(evidenceKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate evidence precompile: %w", err))
	}

	// Stateless precompiles
	precompiles[bech32Precompile.Address()] = bech32Precompile
	precompiles[p256Precompile.Address()] = p256Precompile

	// Stateful precompiles
	precompiles[stakingPrecompile.Address()] = stakingPrecompile
	precompiles[distributionPrecompile.Address()] = distributionPrecompile
	precompiles[ibcTransferPrecompile.Address()] = ibcTransferPrecompile
	precompiles[bankPrecompile.Address()] = bankPrecompile
	precompiles[govPrecompile.Address()] = govPrecompile
	precompiles[slashingPrecompile.Address()] = slashingPrecompile
	precompiles[evidencePrecompile.Address()] = evidencePrecompile

	return precompiles
}
```

## Step 6: Update app.go wiring to Include EVM Modules

Modify your `app/app.go` file to:

1. Add EVM imports:
```go
import (
    // Add these imports
    "math/big"
    "cosmossdk.io/math"
    ante "github.com/cosmos/evm/ante"
    evmante "github.com/cosmos/evm/ante/evm"
    evmencoding "github.com/cosmos/evm/encoding"
    srvflags "github.com/cosmos/evm/server/flags"
    evmtypes "github.com/cosmos/evm/types"
    evmutils "github.com/cosmos/evm/utils"
    "github.com/cosmos/evm/x/erc20"
    erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
    erc20types "github.com/cosmos/evm/x/erc20/types"
    "github.com/cosmos/evm/x/feemarket"
    feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
    feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
    evm "github.com/cosmos/evm/x/vm"
    _ "github.com/cosmos/evm/x/vm/core/tracers/js"
    _ "github.com/cosmos/evm/x/vm/core/tracers/native"
    "github.com/cosmos/evm/x/vm/core/vm"
    evmkeeper "github.com/cosmos/evm/x/vm/keeper"
    evmtypes "github.com/cosmos/evm/x/vm/types"
    
    // Replace default transfer with EVM's transfer
    transfer "github.com/cosmos/evm/x/ibc/transfer"
    ibctransferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
)
```

2. Add EVM module to account permissions:
```go
var maccPerms = map[string][]string{
    // Add these entries
    evmtypes.ModuleName:          {authtypes.Minter, authtypes.Burner},
    feemarkettypes.ModuleName:    nil,
    erc20types.ModuleName:        {authtypes.Minter, authtypes.Burner},
}
```

3. Update the app struct to include EVM keepers:
```go
type ChainApp struct {
    // Add these fields
    FeeMarketKeeper     feemarketkeeper.Keeper
    EVMKeeper           *evmkeeper.Keeper
    Erc20Keeper         erc20keeper.Keeper
    
    // ... existing fields
}
```

4. Update the NewChainApp constructor to include the EVMOptionsFn parameter:
```go
func NewChainApp(
    // ... existing params
    evmAppOptions EVMOptionsFn, // Add this parameter
    // ... other params
) *ChainApp {
```

5. Update encoding configuration to use EVM encoding:
```go
// Replace existing encoding setup with:
encodingConfig := evmencoding.MakeConfig()
interfaceRegistry := encodingConfig.InterfaceRegistry
appCodec := encodingConfig.Codec
legacyAmino := encodingConfig.Amino
txConfig := encodingConfig.TxConfig
```

6. Call EVM App options:
```go
// Add after encoder has been set
if err := evmAppOptions(bApp.ChainID()); err != nil {
    // initialize the EVM application configuration
    panic(fmt.Errorf("failed to initialize EVM app configuration: %w", err))
}
```

7. Add EVM store keys:
```go
keys := storetypes.NewKVStoreKeys(
    // Add these keys
    evmtypes.StoreKey,
    feemarkettypes.StoreKey,
    erc20types.StoreKey,
)

tkeys := storetypes.NewTransientStoreKeys(
    // Add these keys
    evmtypes.TransientKey,
    feemarkettypes.TransientKey,
)
```

8. Initialize EVM keepers:
```go
app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
    appCodec,
    authtypes.NewModuleAddress(govtypes.ModuleName),
    keys[feemarkettypes.StoreKey],
    tkeys[feemarkettypes.TransientKey],
    app.GetSubspace(feemarkettypes.ModuleName),
)

tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

// NOTE: it's required to set up the EVM keeper before the ERC-20 keeper, because it is used in its instantiation.
app.EVMKeeper = evmkeeper.NewKeeper(
    appCodec,
    keys[evmtypes.StoreKey],
    tkeys[evmtypes.TransientKey],
    authtypes.NewModuleAddress(govtypes.ModuleName),
    app.AccountKeeper,
    app.BankKeeper,
    app.StakingKeeper,
    app.FeeMarketKeeper,
    &app.Erc20Keeper,
    tracer, app.GetSubspace(evmtypes.ModuleName),
)

app.Erc20Keeper = erc20keeper.NewKeeper(
    keys[erc20types.StoreKey],
    appCodec,
    authtypes.NewModuleAddress(govtypes.ModuleName),
    app.AccountKeeper,
    app.BankKeeper,
    app.EVMKeeper,
    app.StakingKeeper,
    app.AuthzKeeper,
    &app.TransferKeeper,
)

// Configure EVM precompiles
corePrecompiles := NewAvailableStaticPrecompiles(
    *app.StakingKeeper,
    app.DistrKeeper,
    app.BankKeeper,
    app.Erc20Keeper,
    app.AuthzKeeper,
    app.TransferKeeper,
    app.IBCKeeper.ChannelKeeper,
    app.EVMKeeper,
    app.GovKeeper,
    app.SlashingKeeper,
    app.EvidenceKeeper,
)
app.EVMKeeper.WithStaticPrecompiles(
    corePrecompiles,
)
```

7. Update TransferKeeper initialization to include ERC20Keeper.

Note that the IBC keeper is an extended version from the `x/ibc` module from Cosmos EVM:

```go
// Remove
// import "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
// import ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"

// Add 
import transfer "github.com/cosmos/evm/x/ibc/transfer"
import ibctransferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
```

```go
// Add Erc20Keeper to TransferKeeper params
app.TransferKeeper = ibctransferkeeper.NewKeeper(
    appCodec,
    keys[ibctransfertypes.StoreKey],
    app.GetSubspace(ibctransfertypes.ModuleName),
    app.IBCKeeper.ChannelKeeper,
    app.IBCKeeper.ChannelKeeper,
    &app.IBCKeeper.PortKeeper,
    app.AccountKeeper,
    app.BankKeeper,
    scopedTransferKeeper,
    app.Erc20Keeper,  // Add this
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)
```

8. Add EVM modules to app modules:
```go
app.ModuleManager = module.NewManager(
    // ... existing modules
    // Add these modules
    evm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.GetSubspace(evmtypes.ModuleName)),
    feemarket.NewAppModule(app.FeeMarketKeeper, app.GetSubspace(feemarkettypes.ModuleName)),
    erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper, app.GetSubspace(erc20types.ModuleName)),
    // ... existing modules
)
```

9. Update module ordering:
```go
app.ModuleManager.SetOrderBeginBlockers(
    minttypes.ModuleName,
    erc20types.ModuleName,
    feemarkettypes.ModuleName,
    evmtypes.ModuleName, // NOTE: EVM BeginBlocker must come after FeeMarket BeginBlocker
    // ... existing modules
)

// Add to SetOrderEndBlockers
app.ModuleManager.SetOrderEndBlockers(
    // ... existing modules
    evmtypes.ModuleName,
    feemarkettypes.ModuleName,
    erc20types.ModuleName,
    // ... existing modules
)
// ... 

// Add to SetOrderInitGenesis
genesisModuleOrder := []string{
	// ... existing modules
	evmtypes.ModuleName,
	feemarkettypes.ModuleName, // feemarket module must be initialized before genutil module
	erc20types.ModuleName,
	// ... existing modules
}

```

10. Update Ante handler options:
```go
options := chainante.HandlerOptions{
    // Add these options
    FeeMarketKeeper: app.FeeMarketKeeper,
	
    EvmKeeper:              app.EVMKeeper,
    ExtensionOptionChecker: evmtypes.HasDynamicFeeExtensionOption,
    SigGasConsumer:         evmante.SigVerificationGasConsumer,
    MaxTxGasWanted:         cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted)),
    TxFeeChecker:           evmevmante.NewDynamicFeeChecker(app.FeeMarketKeeper),
    // ... existing options
}
```

11. Update the DefaultGenesis method to include EVM genesis:
```go
func (a *ChainApp) DefaultGenesis() map[string]json.RawMessage {
    genesis := a.BasicModuleManager.DefaultGenesis(a.appCodec)

    // Add mint denom configuration
    mintGenState := minttypes.DefaultGenesisState()
    mintGenState.Params.MintDenom = BaseDenom
    genesis[minttypes.ModuleName] = a.appCodec.MustMarshalJSON(mintGenState)

    // Add EVM genesis configuration
    evmGenState := evmtypes.DefaultGenesisState()
    evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
    genesis[evmtypes.ModuleName] = a.appCodec.MustMarshalJSON(evmGenState)

    // Add ERC20 genesis configuration
    erc20GenState := erc20types.DefaultGenesisState()
    erc20GenState.TokenPairs = ExampleTokenPairs
    erc20GenState.Params.NativePrecompiles = append(erc20GenState.Params.NativePrecompiles, WTokenContractMainnet)
    genesis[erc20types.ModuleName] = a.appCodec.MustMarshalJSON(erc20GenState)

    return genesis
}
```

12. Update blocked addresses to include precompiles:
```go
func BlockedAddresses() map[string]bool {
    // Add after existing code:
    blockedPrecompilesHex := evmtypes.AvailableStaticPrecompiles
    for _, addr := range vm.PrecompiledAddressesBerlin {
        blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
    }

    for _, precompile := range blockedPrecompilesHex {
        blockedAddrs[evmutils.EthHexToCosmosAddr(precompile).String()] = true
    }
    
    return blockedAddrs
}
```

13. Update params keeper subspaces:
```go
// Add these lines:
paramsKeeper.Subspace(evmtypes.ModuleName)
paramsKeeper.Subspace(feemarkettypes.ModuleName)
paramsKeeper.Subspace(erc20types.ModuleName)
```

16. Update the GenesisState Type

```go
func (app *ChainApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
    // Change this line
    var genesisState evmtypes.GenesisState
    // ... rest remains the same
}
```

## Step 7: Update Every Place the EVMAppOptions is Used

Make sure the EVMAppOptions parameter is passed to NewChainApp in all files:

Example: `app/test_helpers.go`
```go
func setup(
    // ...
) {
    app := NewChainApp(
        // ...
        appOptions,
        EVMAppOptions,
        // ...
    )
}
```

## Step 8: Create EVM Ante Handler Files

The EVM requires a different set of AnteHandlers compared to Cosmos. To handle those transactions,
set up the handlers as follows in an `ante` folder:

Create a `handler_options.go`:

```go
package ante

import (
	"context"

	addresscodec "cosmossdk.io/core/address"
	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"
)

// BankKeeper defines the contract needed for supply related APIs (noalias)
type BankKeeper interface {
	IsSendEnabledCoins(ctx context.Context, coins ...sdk.Coin) error
	SendCoins(ctx context.Context, from, to sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
}

type AccountKeeper interface {
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, account sdk.AccountI)
	RemoveAccount(ctx context.Context, account sdk.AccountI)
	GetParams(ctx context.Context) (params authtypes.Params)
	GetSequence(ctx context.Context, addr sdk.AccAddress) (uint64, error)
	AddressCodec() addresscodec.Codec
}

// HandlerOptions defines the list of module keepers required to run the EVM
// AnteHandler decorators.
type HandlerOptions struct {
	Cdc                    codec.BinaryCodec
	AccountKeeper          AccountKeeper
	BankKeeper             BankKeeper
	FeegrantKeeper         ante.FeegrantKeeper
	ExtensionOptionChecker ante.ExtensionOptionChecker
	SignModeHandler        *txsigning.HandlerMap
	SigGasConsumer         func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error
	TxFeeChecker           ante.TxFeeChecker // safe to be nil

	MaxTxGasWanted  uint64
	FeeMarketKeeper anteinterfaces.FeeMarketKeeper
	EvmKeeper       anteinterfaces.EVMKeeper

	IBCKeeper     *ibckeeper.Keeper
	CircuitKeeper *circuitkeeper.Keeper
}

// Validate checks if the keepers are defined
func (options HandlerOptions) Validate() error {
	if options.Cdc == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "codec is required for AnteHandler")
	}
	if options.AccountKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.SigGasConsumer == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "signature gas consumer is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.CircuitKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "circuit keeper is required for ante builder")
	}

	if options.TxFeeChecker == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "tx fee checker is required for AnteHandler")
	}
	if options.FeeMarketKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "fee market keeper is required for AnteHandler")
	}
	if options.EvmKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "evm keeper is required for AnteHandler")
	}

	return nil
}
```

Create an `ante_evm.go` file to handle EVM transactions:

```go
package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmante "github.com/cosmos/evm/ante/evm"
)

// newMonoEVMAnteHandler creates the sdk.AnteHandler implementation for the EVM transactions.
func newMonoEVMAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return sdk.ChainAnteDecorators(
		evmante.NewEVMMonoDecorator(
			options.AccountKeeper,
			options.FeeMarketKeeper,
			options.EvmKeeper,
			options.MaxTxGasWanted,
		),
	)
}
```

Move the existing Cosmos AnteHandler instantiation into a new file, `ante_cosmos.go`:
```go
package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	evmcosmosante "github.com/cosmos/evm/ante/cosmos"
	evmante "github.com/cosmos/evm/ante/evm"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	circuitante "cosmossdk.io/x/circuit/ante"
	ibcante "github.com/cosmos/ibc-go/v8/modules/core/ante"
	poaante "github.com/strangelove-ventures/poa/ante"
)

// newCosmosAnteHandler creates the default ante handler for Cosmos transactions
func NewCosmosAnteHandler(options HandlerOptions) sdk.AnteHandler {
	poaDoGenTxRateValidation := false
	poaRateFloor := sdkmath.LegacyMustNewDecFromStr("0.10")
	poaRateCeil := sdkmath.LegacyMustNewDecFromStr("0.50")

	return sdk.ChainAnteDecorators(
		evmcosmosante.NewRejectMessagesDecorator(), // reject MsgEthereumTxs
		evmcosmosante.NewAuthzLimiterDecorator( // disable the Msg types that cannot be included on an authz.MsgExec msgs field
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
		),

		ante.NewSetUpContextDecorator(),
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		evmcosmosante.NewMinGasPriceDecorator(options.FeeMarketKeeper, options.EvmKeeper),
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker),
		// SetPubKeyDecorator must be called before all signature verification decorators
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer),
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler),
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
		evmante.NewGasWantedDecorator(options.EvmKeeper, options.FeeMarketKeeper),
		poaante.NewPOADisableStakingDecorator(),
		poaante.NewPOADisableWithdrawDelegatorRewards(),
		poaante.NewCommissionLimitDecorator(poaDoGenTxRateValidation, poaRateFloor, poaRateCeil),
	)
}
```

Finally, tie this all together into a global AnteHandler in `ante.go` to handle both types of transactions:
```go
package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// NewAnteHandler returns an ante handler responsible for attempting to route an
// Ethereum or SDK transaction to an internal ante handler for performing
// transaction-level processing (e.g. fee payment, signature verification) before
// being passed onto it's respective handler.
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/os.evm.v1.ExtensionOptionsEthereumTx":
					// handle as *evmtypes.MsgEthereumTx
					anteHandler = newMonoEVMAnteHandler(options)
				case "/os.types.v1.ExtensionOptionDynamicFeeTx":
					// cosmos-sdk tx with dynamic fee extension
					anteHandler = NewCosmosAnteHandler(options)
				default:
					return ctx, errorsmod.Wrapf(
						errortypes.ErrUnknownExtensionOptions,
						"rejecting tx with unsupported extension option: %s", typeURL,
					)
				}

				return anteHandler(ctx, tx, sim)
			}
		}

		// handle as totally normal Cosmos SDK tx
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = NewCosmosAnteHandler(options)
		default:
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid transaction type: %T", tx)
		}

		return anteHandler(ctx, tx, sim)
	}
}
```

## Step 9: Update Command Files

Update `cmd/simd/commands.go`:
```go
// Add imports
evmserverconfig "github.com/cosmos/evm/server/config"
evmcmd "github.com/cosmos/evm/client"
evmserver "github.com/cosmos/evm/server"
srvflags "github.com/cosmos/evm/server/flags"

// Update CustomAppConfig struct
type CustomAppConfig struct {
    serverconfig.Config
    
    // Add these fields
    EVM     evmserverconfig.EVMConfig
    JSONRPC evmserverconfig.JSONRPCConfig
    TLS     evmserverconfig.TLSConfig
}

// Update initAppConfig
func initAppConfig() (string, interface{}) {
    // add the custom app config:
    customAppConfig := CustomAppConfig{
        Config:  *srvCfg,
        EVM:     *evmserverconfig.DefaultEVMConfig(),
        JSONRPC: *evmserverconfig.DefaultJSONRPCConfig(),
        TLS:     *evmserverconfig.DefaultTLSConfig(),
    }
    
    // Add EVM template to existing config
    customAppTemplate += evmserverconfig.DefaultEVMConfigTemplate
    
    return customAppTemplate, customAppConfig
}

// Update initRootCmd
func initRootCmd(
    // ...
) {
    // Replace server.AddCommands with the following to
	// add EVM Comet commands to start server, etc.:
    evmserver.AddCommands(
        rootCmd,
        evmserver.NewDefaultStartOptions(newApp, app.DefaultNodeHome),
        appExport,
        addModuleInitFlags,
    )
    
    // Add EVM key commands
    rootCmd.AddCommand(
		// ... existing commands
        evmcmd.KeyCommands(app.DefaultNodeHome, true),
    )
    
    // Add tx flags
    var err error
    rootCmd, err = srvflags.AddTxFlags(rootCmd)
    if err != nil {
        panic(err)
    }
}
```

## Step 10: Update root.go to Use EVM-Compatible Keyring

Update `cmd/simd/root.go`:
```go
import (
    // Add import
    evmkeyring "github.com/cosmos/evm/crypto/keyring"
)

tempApp := app.NewChainApp(
    log.NewNopLogger(), dbm.NewMemDB(), nil, false, simtestutil.NewAppOptionsWithFlagHome(tempDir()),
    app.NoOpEvmosOptions, // IMPORTANT: ensure that this is the no-op option
)

func NewRootCmd() *cobra.Command {
    // In client context setup
    clientCtx = clientCtx.
		// ... existing options
        WithBroadcastMode(flags.FlagBroadcastMode). // Add this
        WithKeyringOptions(evmkeyring.Option()). // Add this
        WithLedgerHasProtobuf(true).               // Add this
}
```

## Step 11: Update Chain Registry Files

Update `chain_registry.json`:
```json
{
  "chain_type": "ethereum",
  "key_algos": [
    "eth_secp256k1"
  ],
  "slip44": 60
}
```

## Step 12: Final Checks and Running Your Local Chain

TODO: Refer to the following script for an example for how to set up a local testnet: 

## Troubleshooting Tips

- Make sure all keeper initializations are in the correct order - the EVM keeper must be initialized before the ERC20 keeper.

- Check that all necessary parameters are being passed to the keepers (especially the Erc20Keeper to TransferKeeper).

- Verify that the IBC Transfer keepers being used are the ones from Cosmos EVM.

- If you get errors about unknown extension options, make sure your ante handlers are properly configured to recognize EVM transactions.

- Verify that the key algorithm is set to `eth_secp256k1` in all relevant places.

- If you're having trouble with the chain not recognizing EVM transaction formats, verify that the encoding config is using `evmencoding.MakeConfig()`.
