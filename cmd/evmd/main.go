package main

import (
	"evmd"
	"fmt"
	"os"

	"evmd/cmd/evmd/cmd"
	evmdconfig "evmd/cmd/evmd/config"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func main() {
	setupSDKConfig()

	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "evmd", evmd.DefaultNodeHome); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}

func setupSDKConfig() {
	config := sdk.GetConfig()
	evmdconfig.SetBech32Prefixes(config)
	config.Seal()
}
