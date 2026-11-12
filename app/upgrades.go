package app

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	const planName = "v1.6.2.1g"

	app.UpgradeKeeper.SetUpgradeHandler(
		planName,
		func(
			ctx context.Context,
			plan upgradetypes.Plan,
			fromVM module.VersionMap,
		) (module.VersionMap, error) {
			// Run standard module migrations first, just in case.
			m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return m, err
			}

			// Unwrap the Cosmos SDK context for custom migration logic (only necessary to debug gas)
			sdkCtx := sdk.UnwrapSDKContext(ctx)

			if err := app.recoverAirdropToCommunityPool(sdkCtx); err != nil {
				return m, err
			}

			return m, nil
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Errorf("failed to read upgrade info from disk: %w", err))
	}

	if app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return false
	}

	if upgradeInfo.Name != planName {
		return false
	}

	// this plan does not add or delete stores, so no store loader is needed.
	return true
}
