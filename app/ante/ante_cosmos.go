package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	evmoscosmosante "github.com/cosmos/evm/ante/cosmos"
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
		evmoscosmosante.NewRejectMessagesDecorator(), // reject MsgEthereumTxs
		evmoscosmosante.NewAuthzLimiterDecorator( // disable the Msg types that cannot be included on an authz.MsgExec msgs field
			sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
			sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}),
		),

		ante.NewSetUpContextDecorator(),
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),
		evmoscosmosante.NewMinGasPriceDecorator(options.FeeMarketKeeper, options.EvmKeeper),
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
