package baseline

import "github.com/ante-rg/ante-app/api/internal/fakeartifacts"

const datasetVersion = "TSI-100-v1"

func Card(version string, ds fakeartifacts.Dataset) fakeartifacts.Card {
	return fakeartifacts.Card{
		ModelVersion:    version,
		Synthetic:       false,
		DefaultDay:      ds.DefaultDay,
		DefaultCapacity: 10,
		DatasetVersion:  datasetVersion,
		Summary: "A rule baseline, not a model. The score rises with the player's deposits in the last 30 days, " +
			"as deposits / (deposits + 2,000 €), using only data up to that day. Night and loss-chasing reasons are " +
			"shown as context and do not change the score. It is a signal for human review, never a diagnosis.",
		LabelDefinition: "No label exists yet. Chase sessions follow the data science team's proxy " +
			"(2 hours or longer, 3 or more deposits, net loss). The session and chase definitions are provisional " +
			"until the data science team freezes them.",
		IntendedUse: "Running and demonstrating the app on real anonymised behaviour until ante-research publishes a model.",
		DataProtection: "TSI-100-v1: 100 anonymised accounts with archive-local pseudonyms. Time is only relative to " +
			"each account's first event. No rows from the export are kept in the repository.",
		EthicalRisks: "Deposits alone miss other signs of harm and can flag high-spending players who are not at risk. " +
			"The score must not be read as a diagnosis.",
		Limits: []string{
			"Deposit times are technical, not confirmed behaviour time.",
			"balance_after_eur is not a verified own-funds balance.",
			"Night uses the recorded local hour; its timezone is not stated.",
			"Session and chase definitions are provisional.",
			"A session that runs past midnight is counted, deposits included, on the day it starts.",
			"Net result covers completed ordinary rounds only; free-spin and bonus wins are not included.",
			"No evaluation, because the export has no labels.",
		},
		Metrics: []fakeartifacts.CardMetric{},
	}
}
