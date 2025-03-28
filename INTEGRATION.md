# Step-by-Step EVM Integration Guide for Cosmos SDK Applications

This comprehensive guide outlines the exact steps needed to integrate Ethereum Virtual Machine (EVM) functionality into your Cosmos SDK application. Follow these instructions to add complete Ethereum compatibility to your blockchain.

## 1. Update Dependencies

First, update your `go.mod` file with the required dependencies:

```go
require (
    // Existing dependencies...
    
    github.com/cosmos/evm v0.1.1-0.20250328143818-59c573a37f8b
    github.com/ethereum/go-ethereum v1.10.26-evmos-rc4 // Use Cosmos fork of go-ethereum
    
    // Other EVM-related dependencies
    github.com/cosmos/ibc-go/modules/capability v1.0.1
    github.com/cosmos/ibc-go/v8 v8.7.0
)

replace (
    // Use Cosmos fork of go-ethereum
    github.com/ethereum/go-ethereum => github.com/cosmos/go-ethereum v1.10.26-evmos-rc4
)
```

Run:
```bash
go mod tidy
```

## 2. Define Chain Configuration

Create `config.go` to configure your chain's basic parameters:

```go
package yourapp

import (
    "github.com/cosmos/evm/types"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
    // Bech32 prefixes
    Bech32Prefix = "cosmos" // Change to your chain's prefix
    
    // Denom settings
    DisplayDenom = "atom" // Change to your token name
    BaseDenom = "aatom"   // Change to your base denom
    BaseDenomUnit = 18    // Usually 18 for EVM compatibility
)

// SetBech32Prefixes sets the global prefixes for address serialization
func SetBech32Prefixes(config *sdk.Config) {
    config.SetBech32PrefixForAccount(Bech32Prefix, Bech32Prefix + sdk.PrefixPublic)
    config.SetBech32PrefixForValidator(Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator,
                                      Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic)
    config.SetBech32PrefixForConsensusNode(Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus,
                                          Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic)
}

// SetBip44CoinType sets the global coin type for HD derivation
func SetBip44CoinType(config *sdk.Config) {
    config.SetCoinType(types.Bip44CoinType) // 60 for Ethereum
    config.SetPurpose(sdk.Purpose)
    config.SetFullFundraiserPath(types.BIP44HDPath) // m/44'/60'/0'/0/0
}
```

## 3. Create EVM Configuration

Create an `EvmAppOptions` function in a new file named `constants.go`:

```go
package yourapp

import (
    "fmt"
    "strings"
    
    evmtypes "github.com/cosmos/evm/x/vm/types"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
    // Chain IDs - define your chain IDs here
    YourChainID = "yourchain_1"
    
    // Other constants
    ChainDenom = "atoken"
    DisplayDenom = "TOKEN"
)

// ChainsCoinInfo maps chain IDs to their coin configuration
var ChainsCoinInfo = map[string]evmtypes.EvmCoinInfo{
    YourChainID: {
        Denom:        ChainDenom,
        DisplayDenom: DisplayDenom,
        Decimals:     evmtypes.EighteenDecimals, // Use 18 decimals for EVM compatibility
    },
}

var sealed = false

// EvmAppOptions configures the EVM module for your chain
func EvmAppOptions(chainID string) error {
    if sealed {
        return nil
    }

    id := strings.Split(chainID, "-")[0]
    coinInfo, found := ChainsCoinInfo[id]
    if !found {
        return fmt.Errorf("unknown chain id: %s", id)
    }

    // Set the denom info
    if err := setBaseDenom(coinInfo); err != nil {
        return err
    }

    baseDenom, err := sdk.GetBaseDenom()
    if err != nil {
        return err
    }

    // Configure EVM with Ethereum chain settings
    ethCfg := evmtypes.DefaultChainConfig(chainID)
    
    err = evmtypes.NewEVMConfigurator().
        WithChainConfig(ethCfg).
        WithEVMCoinInfo(baseDenom, uint8(coinInfo.Decimals)).
        Configure()
    if err != nil {
        return err
    }

    sealed = true
    return nil
}

// setBaseDenom registers the display denom and base denom
func setBaseDenom(ci evmtypes.EvmCoinInfo) error {
    if err := sdk.RegisterDenom(ci.DisplayDenom, sdk.OneDec()); err != nil {
        return err
    }
    
    return sdk.RegisterDenom(ci.Denom, sdk.NewDecWithPrec(1, int64(ci.Decimals)))
}
```

## 4. Define Store Keys and KVStores

In your `app.go`, add these store keys:

```go
keys := storetypes.NewKVStoreKeys(
    // Existing keys...
    
    // EVM store keys
    evmtypes.StoreKey, 
    feemarkettypes.StoreKey, 
    erc20types.StoreKey,
)

tkeys := storetypes.NewTransientStoreKeys(
    paramstypes.TStoreKey, 
    evmtypes.TransientKey, 
    feemarkettypes.TransientKey,
)

memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
```

## 5. Define Required Keepers

Add these keeper definitions to your app struct:

```go
type YourApp struct {
    // Existing fields...
    
    // IBC keepers
    IBCKeeper            *ibckeeper.Keeper
    TransferKeeper       transferkeeper.Keeper
    scopedIBCKeeper      capabilitykeeper.ScopedKeeper
    ScopedTransferKeeper capabilitykeeper.ScopedKeeper
    
    // Cosmos EVM keepers
    FeeMarketKeeper feemarketkeeper.Keeper
    EVMKeeper       *evmkeeper.Keeper
    Erc20Keeper     erc20keeper.Keeper
    
    // Other fields...
}
```

## 6. Update Module Permissions

Update your module account permissions:

```go
maccPerms = map[string][]string{
    // Existing permissions...
    
    // Cosmos EVM modules
    evmtypes.ModuleName:       {authtypes.Minter, authtypes.Burner},
    feemarkettypes.ModuleName: nil,
    erc20types.ModuleName:     {authtypes.Minter, authtypes.Burner},
    
    // IBC-related
    ibctransfertypes.ModuleName: {authtypes.Minter, authtypes.Burner},
}
```

## 7. Initialize Capability Keeper

In your app constructor:

```go
app.CapabilityKeeper = capabilitykeeper.NewKeeper(
    appCodec, 
    keys[capabilitytypes.StoreKey], 
    memKeys[capabilitytypes.MemStoreKey],
)

// Set up capability scopes for IBC modules
app.scopedIBCKeeper = app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
app.ScopedTransferKeeper = app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)

// Seal capability keeper after scopes are set
app.CapabilityKeeper.Seal()
```

## 8. Initialize EVM Keepers

Initialize the EVM, FeeMarket, and ERC20 keepers:

```go
// Fee Market Keeper must be initialized before EVM keeper
app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
    appCodec, 
    authtypes.NewModuleAddress(govtypes.ModuleName),
    keys[feemarkettypes.StoreKey],
    tkeys[feemarkettypes.TransientKey],
    app.GetSubspace(feemarkettypes.ModuleName),
)

// Get tracer config
tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

// Initialize EVM Keeper
app.EVMKeeper = evmkeeper.NewKeeper(
    appCodec, 
    keys[evmtypes.StoreKey], 
    tkeys[evmtypes.TransientKey],
    authtypes.NewModuleAddress(govtypes.ModuleName),
    app.AccountKeeper,
    app.BankKeeper,
    app.StakingKeeper,
    app.FeeMarketKeeper,
    &app.Erc20Keeper, // Initialized later but referenced here
    tracer, 
    app.GetSubspace(evmtypes.ModuleName),
)

// Initialize ERC20 Keeper
app.Erc20Keeper = erc20keeper.NewKeeper(
    keys[erc20types.StoreKey],
    appCodec,
    authtypes.NewModuleAddress(govtypes.ModuleName),
    app.AccountKeeper,
    app.BankKeeper,
    app.EVMKeeper,
    app.StakingKeeper,
    app.AuthzKeeper,
    &app.TransferKeeper, // Will be initialized later
)
```

## 9. Initialize IBC and ICS20 Modules

Set up the IBC and ICS20 transfer module:

```go
// Create IBC Keeper
app.IBCKeeper = ibckeeper.NewKeeper(
    appCodec,
    keys[ibcexported.StoreKey],
    app.GetSubspace(ibcexported.ModuleName),
    app.StakingKeeper,
    app.UpgradeKeeper,
    app.scopedIBCKeeper,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

// Initialize IBC transfer keeper AFTER ERC20 keeper
app.TransferKeeper = transferkeeper.NewKeeper(
    appCodec, 
    keys[ibctransfertypes.StoreKey], 
    app.GetSubspace(ibctransfertypes.ModuleName),
    nil, // No ICS4 wrapper
    app.IBCKeeper.ChannelKeeper, 
    app.IBCKeeper.PortKeeper,
    app.AccountKeeper, 
    app.BankKeeper, 
    app.ScopedTransferKeeper,
    app.Erc20Keeper, // Add ERC20 Keeper for ERC20 transfers
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

// Create Transfer Stack
transferModule := transfer.NewAppModule(app.TransferKeeper)
var transferStack porttypes.IBCModule
transferStack = transfer.NewIBCModule(app.TransferKeeper)
transferStack = erc20.NewIBCMiddleware(app.Erc20Keeper, transferStack)

// Set IBC Router
ibcRouter := porttypes.NewRouter()
ibcRouter.AddRoute(ibctransfertypes.ModuleName, transferStack)
app.IBCKeeper.SetRouter(ibcRouter)
```

## 10. Add Precompiled Contracts

First, create a precompiles.go file:

```go
package yourapp

import (
    "fmt"
    "maps"

    "github.com/ethereum/go-ethereum/common"

    bankprecompile "github.com/cosmos/evm/precompiles/bank"
    "github.com/cosmos/evm/precompiles/bech32"
    distprecompile "github.com/cosmos/evm/precompiles/distribution"
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

    authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
    bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
    distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
    govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
    slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
    stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

const bech32PrecompileBaseGas = 6_000

// NewAvailableStaticPrecompiles returns the complete list of EVM precompiled contracts
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
) map[common.Address]vm.PrecompiledContract {
    // Start with standard Ethereum precompiles
    precompiles := maps.Clone(vm.PrecompiledContractsBerlin)

    // Add Cosmos-specific precompiles
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
    
    // Register the precompiles
    precompiles[bech32Precompile.Address()] = bech32Precompile
    precompiles[stakingPrecompile.Address()] = stakingPrecompile
    precompiles[distributionPrecompile.Address()] = distributionPrecompile
    precompiles[ibcTransferPrecompile.Address()] = ibcTransferPrecompile
    precompiles[bankPrecompile.Address()] = bankPrecompile
    precompiles[govPrecompile.Address()] = govPrecompile
    
    return precompiles
}
```

Then, add the precompiles to your EVMKeeper:

```go
app.EVMKeeper.WithStaticPrecompiles(
    NewAvailableStaticPrecompiles(
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
    ),
)
```

## 11. Create AnteHandler Package

Create a new directory `ante` with the following files:

### a. ante/ante.go

```go
package ante

import (
    errorsmod "cosmossdk.io/errors"

    sdk "github.com/cosmos/cosmos-sdk/types"
    errortypes "github.com/cosmos/cosmos-sdk/types/errors"
    authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// NewAnteHandler returns an ante handler that routes transactions
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
    return func(ctx sdk.Context, tx sdk.Tx, sim bool) (newCtx sdk.Context, err error) {
        var anteHandler sdk.AnteHandler

        txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
        if ok {
            opts := txWithExtensions.GetExtensionOptions()
            if len(opts) > 0 {
                switch typeURL := opts[0].GetTypeUrl(); typeURL {
                case "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx":
                    // handle as EVM tx
                    anteHandler = newMonoEVMAnteHandler(options)
                case "/cosmos.evm.types.v1.ExtensionOptionDynamicFeeTx":
                    // cosmos-sdk tx with dynamic fee extension
                    anteHandler = newCosmosAnteHandler(options)
                default:
                    return ctx, errorsmod.Wrapf(
                        errortypes.ErrUnknownExtensionOptions,
                        "rejecting tx with unsupported extension option: %s", typeURL,
                    )
                }

                return anteHandler(ctx, tx, sim)
            }
        }

        // Handle as normal Cosmos SDK tx
        switch tx.(type) {
        case sdk.Tx:
            anteHandler = newCosmosAnteHandler(options)
        default:
            return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid transaction type: %T", tx)
        }

        return anteHandler(ctx, tx, sim)
    }
}
```

### b. ante/cosmos_handler.go

```go
package ante

import (
    cosmosante "github.com/cosmos/evm/ante/cosmos"
    evmante "github.com/cosmos/evm/ante/evm"
    evmtypes "github.com/cosmos/evm/x/vm/types"
    ibcante "github.com/cosmos/ibc-go/v8/modules/core/ante"

    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/cosmos/cosmos-sdk/x/auth/ante"
    sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

// newCosmosAnteHandler creates the ante handler for Cosmos transactions
func newCosmosAnteHandler(options HandlerOptions) sdk.AnteHandler {
    return sdk.ChainAnteDecorators(
        cosmosante.NewRejectMessagesDecorator(), // reject MsgEthereumTxs
        cosmosante.NewAuthzLimiterDecorator( // block certain msg types in authz
            sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
            sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
        ),
        ante.NewSetUpContextDecorator(),
        ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
        ante.NewValidateBasicDecorator(),
        ante.NewTxTimeoutHeightDecorator(),
        ante.NewValidateMemoDecorator(options.AccountKeeper),
        cosmosante.NewMinGasPriceDecorator(options.FeeMarketKeeper, options.EvmKeeper),
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
    )
}
```

### c. ante/evm_handler.go

```go
package ante

import (
    evmante "github.com/cosmos/evm/ante/evm"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

// newMonoEVMAnteHandler creates the ante handler for EVM transactions
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

### d. ante/handler_options.go

```go
package ante

import (
    anteinterfaces "github.com/cosmos/evm/ante/interfaces"
    ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

    errorsmod "cosmossdk.io/errors"
    storetypes "cosmossdk.io/store/types"
    txsigning "cosmossdk.io/x/tx/signing"

    "github.com/cosmos/cosmos-sdk/codec"
    errortypes "github.com/cosmos/cosmos-sdk/types/errors"
    "github.com/cosmos/cosmos-sdk/types/tx/signing"
    "github.com/cosmos/cosmos-sdk/x/auth/ante"
    authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// HandlerOptions defines the required keepers for the EVM AnteHandler
type HandlerOptions struct {
    Cdc                    codec.BinaryCodec
    AccountKeeper          anteinterfaces.AccountKeeper
    BankKeeper             anteinterfaces.BankKeeper
    IBCKeeper              *ibckeeper.Keeper
    FeeMarketKeeper        anteinterfaces.FeeMarketKeeper
    EvmKeeper              anteinterfaces.EVMKeeper
    FeegrantKeeper         ante.FeegrantKeeper
    ExtensionOptionChecker ante.ExtensionOptionChecker
    SignModeHandler        *txsigning.HandlerMap
    SigGasConsumer         func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error
    MaxTxGasWanted         uint64
    TxFeeChecker           ante.TxFeeChecker
}

// Validate checks if the keepers are properly initialized
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
    if options.IBCKeeper == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "ibc keeper is required for AnteHandler")
    }
    if options.FeeMarketKeeper == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "fee market keeper is required for AnteHandler")
    }
    if options.EvmKeeper == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "evm keeper is required for AnteHandler")
    }
    if options.SigGasConsumer == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "signature gas consumer is required for AnteHandler")
    }
    if options.SignModeHandler == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "sign mode handler is required for AnteHandler")
    }
    if options.TxFeeChecker == nil {
        return errorsmod.Wrap(errortypes.ErrLogic, "tx fee checker is required for AnteHandler")
    }
    return nil
}
```

## 12. Integrate AnteHandler in Your App

Add this method to your app struct:

```go
func (app *YourApp) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
    options := ante.HandlerOptions{
        Cdc:                    app.appCodec,
        AccountKeeper:          app.AccountKeeper,
        BankKeeper:             app.BankKeeper,
        ExtensionOptionChecker: cosmosevmtypes.HasDynamicFeeExtensionOption,
        EvmKeeper:              app.EVMKeeper,
        FeegrantKeeper:         app.FeegrantKeeper,
        IBCKeeper:              app.IBCKeeper,
        FeeMarketKeeper:        app.FeeMarketKeeper,
        SignModeHandler:        txConfig.SignModeHandler(),
        SigGasConsumer:         evmante.SigVerificationGasConsumer,
        MaxTxGasWanted:         maxGasWanted,
        TxFeeChecker:           cosmosevmante.NewDynamicFeeChecker(app.FeeMarketKeeper),
    }
    if err := options.Validate(); err != nil {
        panic(err)
    }

    app.SetAnteHandler(ante.NewAnteHandler(options))
}
```

Call this in your app constructor:

```go
maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))
app.setAnteHandler(app.txConfig, maxGasWanted)
```

## 13. Add EVM Modules to the Module Manager

Update your module manager:

```go
app.ModuleManager = module.NewManager(
    // Existing modules...
    
    // IBC modules
    ibc.NewAppModule(app.IBCKeeper),
    ibctm.NewAppModule(),
    transferModule,
    
    // Cosmos EVM modules
    vm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.GetSubspace(evmtypes.ModuleName)),
    feemarket.NewAppModule(app.FeeMarketKeeper, app.GetSubspace(feemarkettypes.ModuleName)),
    erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper, app.GetSubspace(erc20types.ModuleName)),
)
```

## 14. Configure Module Execution Order

Set the correct execution order:

```go
// BeginBlockers - The order matters!
app.ModuleManager.SetOrderBeginBlockers(
    capabilitytypes.ModuleName, 
    minttypes.ModuleName,
    
    // IBC modules
    ibcexported.ModuleName, 
    ibctransfertypes.ModuleName,
    
    // Cosmos EVM BeginBlockers - EVM must be after FeeMarket
    erc20types.ModuleName, 
    feemarkettypes.ModuleName,
    evmtypes.ModuleName,
    
    // Other modules...
)

// EndBlockers
app.ModuleManager.SetOrderEndBlockers(
    govtypes.ModuleName, 
    stakingtypes.ModuleName,
    
    // Cosmos EVM EndBlockers 
    evmtypes.ModuleName, 
    erc20types.ModuleName, 
    feemarkettypes.ModuleName,
    
    // Other modules...
)

// Genesis initialization order
app.ModuleManager.SetOrderInitGenesis(
    capabilitytypes.ModuleName,
    // Other modules...
    
    // Cosmos EVM modules - FeeMarket before genutil
    evmtypes.ModuleName,
    feemarkettypes.ModuleName,
    erc20types.ModuleName,
    
    // More modules...
)
```

## 15. Set up Genesis Configuration

Create custom genesis functions for EVM modules in a file named `genesis.go`:

```go
func NewEVMGenesisState() *evmtypes.GenesisState {
    evmGenState := evmtypes.DefaultGenesisState()
    evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
    return evmGenState
}

func NewErc20GenesisState() *erc20types.GenesisState {
    erc20GenState := erc20types.DefaultGenesisState()
    
    // Set up token pairs for your chain
    erc20GenState.TokenPairs = []erc20types.TokenPair{
        {
            Erc20Address:  "0xYourWTokenContractAddress", // Default wrapped token address
            Denom:         YourChainDenom,
            Enabled:       true,
            ContractOwner: erc20types.OWNER_MODULE,
        },
    }
    
    return erc20GenState
}
```

Add this to your app.go file:

```go
// Add this to your app.go file
func (app *YourApp) DefaultGenesis() map[string]json.RawMessage {
    genesis := app.BasicModuleManager.DefaultGenesis(app.appCodec)
    
    // Add custom EVM genesis states
    evmGenState := NewEVMGenesisState()
    genesis[evmtypes.ModuleName] = app.appCodec.MustMarshalJSON(evmGenState)
    
    erc20GenState := NewErc20GenesisState()
    genesis[erc20types.ModuleName] = app.appCodec.MustMarshalJSON(erc20GenState)
    
    // Configure mint module to use your base denom
    mintGenState := minttypes.DefaultGenesisState()
    mintGenState.Params.MintDenom = YourChainDenom
    genesis[minttypes.ModuleName] = app.appCodec.MustMarshalJSON(mintGenState)
    
    return genesis
}
```

## 16. Configure the Command Line Interface

Create a `cmd` directory with the necessary files:

### a. cmd/root.go

```go
package cmd

import (
    "errors"
    "io"
    "os"
    "yourapp" // Import your app package
    
    "github.com/spf13/cast"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    tmcfg "github.com/cometbft/cometbft/config"
    cmtcli "github.com/cometbft/cometbft/libs/cli"

    dbm "github.com/cosmos/cosmos-db"
    cosmosevmcmd "github.com/cosmos/evm/client"
    cosmosevmkeyring "github.com/cosmos/evm/crypto/keyring"
    cosmosevmserver "github.com/cosmos/evm/server"
    cosmosevmserverconfig "github.com/cosmos/evm/server/config"
    srvflags "github.com/cosmos/evm/server/flags"

    "cosmossdk.io/log"
    "cosmossdk.io/store"
    storetypes "cosmossdk.io/store/types"
    confixcmd "cosmossdk.io/tools/confix/cmd"

    "github.com/cosmos/cosmos-sdk/baseapp"
    "github.com/cosmos/cosmos-sdk/client"
    "github.com/cosmos/cosmos-sdk/client/config"
    "github.com/cosmos/cosmos-sdk/client/flags"
    "github.com/cosmos/cosmos-sdk/client/pruning"
    "github.com/cosmos/cosmos-sdk/client/rpc"
    "github.com/cosmos/cosmos-sdk/client/snapshot"
    sdkserver "github.com/cosmos/cosmos-sdk/server"
    servertypes "github.com/cosmos/cosmos-sdk/server/types"
    sdk "github.com/cosmos/cosmos-sdk/types"
    sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
    "github.com/cosmos/cosmos-sdk/types/tx/signing"
    authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
    "github.com/cosmos/cosmos-sdk/x/auth/tx"
    txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
    authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
    genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

type emptyAppOptions struct{}
func (ao emptyAppOptions) Get(_ string) interface{} { return nil }

func NoOpEvmAppOptions(_ string) error {
    return nil
}

// NewRootCmd creates a new root command for your app
func NewRootCmd() *cobra.Command {
    // Create a temporary app to get encoding config
    tempApp := yourapp.NewApp(
        log.NewNopLogger(),
        dbm.NewMemDB(),
        nil,
        true,
        emptyAppOptions{},
        NoOpEvmAppOptions,
    )

    encodingConfig := sdktestutil.TestEncodingConfig{
        InterfaceRegistry: tempApp.InterfaceRegistry(),
        Codec:             tempApp.AppCodec(),
        TxConfig:          tempApp.GetTxConfig(),
        Amino:             tempApp.LegacyAmino(),
    }

    initClientCtx := client.Context{}.
        WithCodec(encodingConfig.Codec).
        WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
        WithTxConfig(encodingConfig.TxConfig).
        WithLegacyAmino(encodingConfig.Amino).
        WithInput(os.Stdin).
        WithAccountRetriever(authtypes.AccountRetriever{}).
        WithBroadcastMode(flags.FlagBroadcastMode).
        WithHomeDir(yourapp.DefaultNodeHome).
        WithViper("").
        // Cosmos EVM specific setup
        WithKeyringOptions(cosmosevmkeyring.Option()).
        WithLedgerHasProtobuf(true)

    rootCmd := &cobra.Command{
        Use:   "yourappd",
        Short: "Your application with EVM support",
        PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
            // Set command outputs
            cmd.SetOut(cmd.OutOrStdout())
            cmd.SetErr(cmd.ErrOrStderr())

            initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
            initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
            if err != nil {
                return err
            }

            initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
            if err != nil {
                return err
            }

            // Set up SIGN_MODE_TEXTUAL support
            if !initClientCtx.Offline {
                enabledSignModes := append(tx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL)
                txConfigOpts := tx.ConfigOptions{
                    EnabledSignModes:           enabledSignModes,
                    TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
                }
                txConfig, err := tx.NewTxConfigWithOptions(
                    initClientCtx.Codec,
                    txConfigOpts,
                )
                if err != nil {
                    return err
                }

                initClientCtx = initClientCtx.WithTxConfig(txConfig)
            }

            if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
                return err
            }

            // Load custom app configuration
            customAppTemplate, customAppConfig := InitAppConfig(yourapp.BaseDenom)
            customTMConfig := initTendermintConfig()

            return sdkserver.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customTMConfig)
        },
    }

    initRootCmd(rootCmd, tempApp)

    autoCliOpts := tempApp.AutoCliOpts()
    initClientCtx, _ = config.ReadFromClientConfig(initClientCtx)
    autoCliOpts.ClientCtx = initClientCtx

    // Add AutoCLI commands
    if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
        panic(err)
    }

    return rootCmd
}
```

### b. cmd/app_config.go

```go
package cmd

import (
    "cosmossdk.io/client"
    tmcfg "github.com/cometbft/cometbft/config"
    serverconfig "github.com/cosmos/cosmos-sdk/server/config"
    cosmosevmserverconfig "github.com/cosmos/evm/server/config"
)

// initTendermintConfig helps to override default CometBFT Config values
func initTendermintConfig() *tmcfg.Config {
    cfg := tmcfg.DefaultConfig()
    
    // Customize if needed:
    // cfg.P2P.MaxNumInboundPeers = 100
    // cfg.P2P.MaxNumOutboundPeers = 40
    
    return cfg
}

// InitAppConfig helps to override default appConfig values
func InitAppConfig(denom string) (string, interface{}) {
    type CustomAppConfig struct {
        serverconfig.Config
        
        EVM     cosmosevmserverconfig.EVMConfig
        JSONRPC cosmosevmserverconfig.JSONRPCConfig
        TLS     cosmosevmserverconfig.TLSConfig
    }
    
    // Set up server config
    srvCfg := serverconfig.DefaultConfig()
    
    // Set minimum gas prices to prevent spam
    srvCfg.MinGasPrices = "0" + denom
    
    customAppConfig := CustomAppConfig{
        Config:  *srvCfg,
        EVM:     *cosmosevmserverconfig.DefaultEVMConfig(),
        JSONRPC: *cosmosevmserverconfig.DefaultJSONRPCConfig(),
        TLS:     *cosmosevmserverconfig.DefaultTLSConfig(),
    }
    
    customAppTemplate := serverconfig.DefaultConfigTemplate +
        cosmosevmserverconfig.DefaultEVMConfigTemplate
    
    return customAppTemplate, customAppConfig
}
```

## 17. Configure the JSON-RPC Server

The JSON-RPC server is essential for Ethereum compatibility. The Cosmos EVM integration provides a JSON-RPC implementation, but you need to properly configure it:

### a. Create a JSON-RPC configuration in app.toml

This is included in the app template you defined in `InitAppConfig()`:

```go
// DefaultJSONRPCConfig returns the default JSON-RPC configuration
func DefaultJSONRPCConfig() *JSONRPCConfig {
    return &JSONRPCConfig{
        Enable:             true,
        API:                "eth,net,web3,txpool,debug",
        Address:            "0.0.0.0:8545",
        WsAddress:          "0.0.0.0:8546",
        GasCap:             25000000,
        EVMTimeout:         5,
        TxFeeCap:           1,
        FilterCap:          200,
        FeeHistoryCap:      100,
        LogsCap:            10000,
        BlockRangeCap:      10000,
        HTTPTimeout:        30,
        HTTPIdleTimeout:    120,
        AllowUnprotectedTxs: false,
    }
}
```

### b. Setup Ethereum Key Derivation

The Cosmos EVM integration handles Ethereum key derivation through `cosmosevmkeyring.Option()` which is included in the client context setup.

Key differences compared to standard Cosmos SDK key derivation:
1. Uses BIP44 coin type 60 (Ethereum's path) instead of 118 (Cosmos)
2. Uses secp256k1 curve for key generation
3. Supports both Cosmos (bech32) and Ethereum (hex) address formats

You've already set this up when initializing your client context:

```go
initClientCtx := client.Context{}.
    // Standard setup...
    // Cosmos EVM specific setup
    WithKeyringOptions(cosmosevmkeyring.Option()).
    WithLedgerHasProtobuf(true)
```

## 18. Ethereum Transaction Signing and Verification

The Cosmos EVM integration supports both standard Cosmos and Ethereum transaction signing. This is handled by:

1. The custom AnteHandler you've implemented
2. The extension options system (`ExtensionOptionsEthereumTx` and `ExtensionOptionDynamicFeeTx`)
3. Special key derivation for Ethereum accounts

Key points about transaction signing:

- Ethereum transactions use RLP encoding and are decoded through the `MsgEthereumTx` message type
- The EVM AnteHandler performs Ethereum-specific signature verification
- Ethereum accounts can be derived from the same mnemonic as Cosmos accounts, but with different derivation paths

## 19. Running and Testing the Integration

After implementing all the components, follow these steps to run and test your EVM-enabled chain:

1. Build your application:
   ```bash
   go build -o yourappd ./cmd/main.go
   ```

2. Initialize a new chain:
   ```bash
   ./yourappd init test --chain-id yourchain_1
   ```

3. Create a key:
   ```bash
   ./yourappd keys add mykey
   ```

4. Add genesis account:
   ```bash
   ./yourappd add-genesis-account mykey 1000000000000000000atoken
   ```

5. Create validator transaction:
   ```bash
   ./yourappd gentx mykey 1000000atoken --chain-id yourchain_1
   ```

6. Collect genesis transactions:
   ```bash
   ./yourappd collect-gentxs
   ```

7. Start the chain:
   ```bash
   ./yourappd start
   ```

8. Test with an Ethereum wallet:
    - Connect MetaMask to `http://localhost:8545`
    - Chain ID: Your chain ID (e.g., `yourchain_1`)
    - Native currency: Your token symbol

## 20. Common Integration Issues and Solutions

1. **EVM Module Registration Problem**:
    - Symptoms: Chain starts but EVM transactions fail with "module not found" errors
    - Solution: Ensure all EVM-related modules are correctly registered in the ModuleManager

2. **Fee-related Errors**:
    - Symptoms: Transactions fail with "insufficient fee" errors
    - Solution: Set correct minimum gas prices in app.toml and check FeeMarket module configuration

3. **AnteHandler Chain Issues**:
    - Symptoms: Transactions fail during validation phase
    - Solution: Verify your AnteHandler is properly registered and all required components are instantiated

4. **JSON-RPC Connection Problems**:
    - Symptoms: Ethereum clients can't connect to your node
    - Solution: Check JSON-RPC configuration, especially the listen addresses and allowed APIs

5. **Transaction Type Detection Failures**:
    - Symptoms: EVM transactions processed as regular Cosmos transactions
    - Solution: Ensure the AnteHandler properly detects and routes different transaction types

## 21. Advanced Customization

After completing the basic integration, you can customize your EVM implementation:

1. **Custom Precompiles**: Implement application-specific Ethereum precompiled contracts
2. **EVM Parameters**: Tune gas prices, block gas limit, and other EVM parameters
3. **Custom Fee Models**: Implement specialized fee mechanisms for your chain
4. **Token Pair Registration**: Add more ERC20 to native token conversions