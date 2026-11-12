package app

import (
	_ "embed"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"
)

//go:embed migration_data/airdropped-addresses.json
var airdroppedAddressesJSON []byte

// Will recover airdropped L1 tokens from all accounts that have a nonce below 1
func (app *App) recoverAirdropToCommunityPool(ctx sdk.Context) error {
	const denom = "el1"
	const logInterval = 10_000

	var sourceAddresses []string

	startTime := time.Now()
	startGas := ctx.GasMeter().GasConsumed()
	startBlockGas := ctx.BlockGasMeter().GasConsumed()

	app.Logger().Info(
		"Community pool migration gas diagnostics",
		"execution_gas_limit", ctx.GasMeter().Limit(),
		"block_gas_limit", ctx.BlockGasMeter().Limit(),
		"execution_gas_start", startGas,
		"block_gas_start", startBlockGas,
	)

	if err := json.Unmarshal(airdroppedAddressesJSON, &sourceAddresses); err != nil {
		return fmt.Errorf(
			"failed to parse community pool migration addresses: %w",
			err,
		)
	}

	totalAddresses := len(sourceAddresses)

	app.Logger().Info(
		"Starting liquid L1 balance transfer to community pool",
		"addresses", totalAddresses,
		"denom", denom,
	)

	addresses := make([]sdk.AccAddress, totalAddresses)
	seen := make(map[string]struct{}, totalAddresses)

	for i, address := range sourceAddresses {
		addr, err := sdk.AccAddressFromBech32(address)
		if err != nil {
			return fmt.Errorf(
				"invalid migration address at index %d (%s): %w",
				i, address, err,
			)
		}

		key := addr.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf(
				"duplicate migration address at index %d: %s",
				i, address,
			)
		}

		seen[key] = struct{}{}
		addresses[i] = addr
	}

	app.Logger().Info(
		"Validated community pool migration addresses",
		"addresses", totalAddresses,
	)

	var (
		processed           int
		accountsTransferred int
		accountsZero        int
		accountsSkipped     int
		totalTransferred    = sdkmath.ZeroInt()
	)

	for i, addr := range addresses {
		account := app.AccountKeeper.GetAccount(ctx, addr)

		if account != nil && account.GetSequence() > 0 {
			accountsSkipped++
		} else {
			balance := app.BankKeeper.GetBalance(ctx, addr, denom)

			if balance.IsZero() {
				accountsZero++
			} else {
				if err := app.DistrKeeper.FundCommunityPool(
					ctx,
					sdk.NewCoins(balance),
					addr,
				); err != nil {
					return fmt.Errorf(
						"failed to transfer balance at address %d/%d (%s): %w",
						i+1, totalAddresses, sourceAddresses[i], err,
					)
				}

				accountsTransferred++
				totalTransferred = totalTransferred.Add(balance.Amount)
			}
		}

		processed++

		if processed%logInterval == 0 || processed == totalAddresses {
			app.Logger().Info(
				"Community pool migration progress",
				"processed", processed,
				"total", totalAddresses,
				"accounts_transferred", accountsTransferred,
				"accounts_zero_balance", accountsZero,
				"accounts_skipped", accountsSkipped,
				"total_transferred", sdk.NewCoin(denom, totalTransferred),
				"elapsed", time.Since(startTime).String(),
				"execution_gas", ctx.GasMeter().GasConsumed()-startGas,
				"execution_gas_limit", ctx.GasMeter().Limit(),
				"block_gas", ctx.BlockGasMeter().GasConsumed()-startBlockGas,
				"block_gas_limit", ctx.BlockGasMeter().Limit(),
			)
		}
	}

	app.Logger().Info(
		"Completed liquid L1 balance transfer to community pool",
		"addresses_processed", processed,
		"accounts_transferred", accountsTransferred,
		"accounts_zero_balance", accountsZero,
		"total_transferred", sdk.NewCoin(denom, totalTransferred),
		"elapsed", time.Since(startTime).String(),
		"execution_gas", ctx.GasMeter().GasConsumed()-startGas,
		"execution_gas_limit", ctx.GasMeter().Limit(),
		"block_gas", ctx.BlockGasMeter().GasConsumed()-startBlockGas,
		"block_gas_limit", ctx.BlockGasMeter().Limit(),
	)

	return nil
}