package baseline

import (
	"fmt"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

const datasetVersion = "ANTE financial CSV v4 (2026-11-23)"

func Card(version string, ds fakeartifacts.Dataset, skipped int) fakeartifacts.Card {
	return fakeartifacts.Card{
		ModelVersion:    version,
		Synthetic:       false,
		DefaultDay:      ds.DefaultDay,
		DefaultCapacity: 10,
		DatasetVersion:  datasetVersion,
		Summary: "A rule baseline, not a model. The score rises with the player's deposits in the last 30 days, " +
			"as deposits / (deposits + 2,000 €), using only data up to that day. Night and loss-chasing reasons are " +
			"shown as context and do not change the score. It is a signal for human review, never a diagnosis.",
		LabelDefinition: "Self-exclusion is marked in the export, but this baseline is not evaluated against it. " +
			"Chase sessions follow the data science team's proxy (2 hours or longer, 3 or more deposits, net loss) " +
			"and are provisional until the team freezes them.",
		IntendedUse: "Running and demonstrating the app on the team's current export until ante-research publishes a model.",
		DataProtection: "Operator account and brand IDs are replaced by pseudonyms and brand codes. Time is only relative to " +
			"each account's first event. No rows from the export are kept in the repository.",
		EthicalRisks: "Deposits alone miss other signs of harm and can flag high-spending players who are not at risk. " +
			"The score must not be read as a diagnosis.",
		Limits: []string{
			"Amounts are converted to EUR at ECB monthly average rates.",
			fmt.Sprintf("%d accounts in RUB, KZT or UAH skipped (no ECB rate).", skipped),
			"Sessions are split after 60 minutes without events.",
			"Night uses the UTC hour; the player's timezone is unknown.",
			"balance_after mixes wallets and is not fully reconciled.",
			"Days are 24-hour periods from the account's first event; a session that crosses into the next day is counted on the day it starts.",
			"No evaluation yet.",
		},
		Metrics: []fakeartifacts.CardMetric{},
	}
}
