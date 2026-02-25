import { For, createSignal } from "solid-js";
import { buildMutationURL } from "vorma/client";
import { api } from "../vorma.bindings.ts";
import type { RouteProps } from "../vorma.gen/index.ts";
import { vormaAppConfig } from "../vorma.gen/index.ts";

type EchoStressCase = {
	caseLabel: string;
	value: string;
	amount: number;
};

type StressResultRow = {
	caseLabel: string;
	simpleEchoMatched: boolean;
	complexEchoMatched: boolean;
	scoreTotalMatched: boolean;
};

type ComplexEchoPayload = {
	title: string;
	flags: boolean[];
	scores: number[];
	metadata: Record<string, string>;
};

const staticStressCases: ReadonlyArray<EchoStressCase> = [
	{ caseLabel: "empty-string", value: "", amount: 0 },
	{ caseLabel: "padding", value: "  left and right  ", amount: 17 },
	{ caseLabel: "symbols", value: "!@#$%^&*()[]{}", amount: -13 },
	{
		caseLabel: "json-looking",
		value: '{"looks":"like-json","still":"string"}',
		amount: 33,
	},
	{ caseLabel: "long", value: "x".repeat(160), amount: 4096 },
];

const generatedStressCases = buildDeterministicStressCases({
	seed: 0x5eedb0b5,
	caseCount: 96,
});

const allStressCases: ReadonlyArray<EchoStressCase> = [
	...staticStressCases,
	...generatedStressCases,
];

export function MutationLab(_props: RouteProps<"/mutation-lab">) {
	const [incrementCount, setIncrementCount] = createSignal<number | null>(
		null,
	);
	const [echoValue, setEchoValue] = createSignal("hello");
	const [echoAmount, setEchoAmount] = createSignal("1");
	const [echoResult, setEchoResult] = createSignal("");
	const [forcedFailureResult, setForcedFailureResult] = createSignal("unset");
	const [slowIncrementRaceResult, setSlowIncrementRaceResult] =
		createSignal("unset");
	const [stressSummary, setStressSummary] = createSignal("not-run");
	const [stressFailureCount, setStressFailureCount] = createSignal(0);
	const [stressRows, setStressRows] = createSignal<StressResultRow[]>([]);

	const redirectActionURL = buildMutationURL(vormaAppConfig, {
		pattern: "/submit-and-redirect",
	});

	async function runIncrement() {
		const response = await api.mutate({ pattern: "/increment-count" });
		if (!response.success) {
			setIncrementCount(null);
			return;
		}
		setIncrementCount(response.data.count);
	}

	async function runReset() {
		const response = await api.mutate({ pattern: "/reset-count" });
		if (!response.success) {
			setIncrementCount(null);
			return;
		}
		setIncrementCount(response.data.count);
	}

	async function runEchoMutation() {
		const numericAmount = Number.parseInt(echoAmount(), 10);
		const response = await api.mutate({
			pattern: "/echo-body",
			input: {
				value: echoValue(),
				amount: Number.isNaN(numericAmount) ? 0 : numericAmount,
			},
		});
		if (!response.success) {
			setEchoResult(`failed:${response.error}`);
			return;
		}
		setEchoResult(`${response.data.value}|${response.data.amount}`);
	}

	async function runForcedFailureMutation() {
		const response = await api.mutate({ pattern: "/always-fail" });
		if (response.success) {
			setForcedFailureResult("unexpected-success");
			return;
		}
		setForcedFailureResult(`failed:${response.error}`);
	}

	async function runSlowIncrementRace() {
		setSlowIncrementRaceResult("running");

		const slowMutationPromise = api.mutate({
			pattern: "/slow-increment",
			input: {
				delayMs: 650,
				tag: "slow",
			},
		});
		await sleepMS(20);
		const fastMutationPromise = api.mutate({
			pattern: "/slow-increment",
			input: {
				delayMs: 30,
				tag: "fast",
			},
		});

		const [slowMutation, fastMutation] = await Promise.all([
			slowMutationPromise,
			fastMutationPromise,
		]);
		if (!slowMutation.success || !fastMutation.success) {
			setSlowIncrementRaceResult("failed");
			return;
		}

		setSlowIncrementRaceResult(
			`${slowMutation.data.tag}:${slowMutation.data.count}|${fastMutation.data.tag}:${fastMutation.data.count}`,
		);
	}

	async function runStressMatrix() {
		setStressSummary("running");
		setStressFailureCount(0);
		setStressRows([]);

		const computedRows: StressResultRow[] = [];
		for (const stressCase of allStressCases) {
			const simpleResponse = await api.mutate({
				pattern: "/echo-body",
				input: {
					value: stressCase.value,
					amount: stressCase.amount,
				},
			});
			if (!simpleResponse.success) {
				computedRows.push({
					caseLabel: stressCase.caseLabel,
					simpleEchoMatched: false,
					complexEchoMatched: false,
					scoreTotalMatched: false,
				});
				continue;
			}

			const complexPayload = buildComplexEchoPayloadForStressCase({
				stressCase,
			});
			const complexResponse = await api.mutate({
				pattern: "/echo-complex-body",
				input: complexPayload,
			});
			if (!complexResponse.success) {
				computedRows.push({
					caseLabel: stressCase.caseLabel,
					simpleEchoMatched: true,
					complexEchoMatched: false,
					scoreTotalMatched: false,
				});
				continue;
			}

			const scoreTotal = complexPayload.scores.reduce(
				(total, score) => total + score,
				0,
			);
			computedRows.push({
				caseLabel: stressCase.caseLabel,
				simpleEchoMatched:
					simpleResponse.data.value === stressCase.value &&
					simpleResponse.data.amount === stressCase.amount,
				complexEchoMatched:
					complexResponse.data.title === complexPayload.title &&
					JSON.stringify(complexResponse.data.flags) ===
						JSON.stringify(complexPayload.flags) &&
					JSON.stringify(complexResponse.data.scores) ===
						JSON.stringify(complexPayload.scores) &&
					JSON.stringify(complexResponse.data.metadata) ===
						JSON.stringify(complexPayload.metadata),
				scoreTotalMatched:
					complexResponse.data.scoreTotal === scoreTotal,
			});
		}

		const failureCount = computedRows.filter(
			(row) => !deriveRowPasses(row),
		).length;
		setStressRows(computedRows);
		setStressFailureCount(failureCount);
		setStressSummary(`completed:${computedRows.length}`);
	}

	return (
		<section id="e2e-mutation-route">
			<button
				id="e2e-increment-button"
				type="button"
				onClick={runIncrement}
			>
				Increment Counter
			</button>
			<button id="e2e-reset-button" type="button" onClick={runReset}>
				Reset Counter
			</button>
			<p id="e2e-increment-result">{incrementCount() ?? "unset"}</p>

			<label for="e2e-echo-value">Echo Value</label>
			<input
				id="e2e-echo-value"
				value={echoValue()}
				onInput={(event) => {
					setEchoValue(event.currentTarget.value);
				}}
			/>

			<label for="e2e-echo-amount">Echo Amount</label>
			<input
				id="e2e-echo-amount"
				value={echoAmount()}
				onInput={(event) => {
					setEchoAmount(event.currentTarget.value);
				}}
			/>

			<button
				id="e2e-echo-button"
				type="button"
				onClick={runEchoMutation}
			>
				Run Echo Mutation
			</button>
			<p id="e2e-echo-result">{echoResult()}</p>

			<button
				id="e2e-forced-failure-button"
				type="button"
				onClick={runForcedFailureMutation}
			>
				Run Forced Failure Mutation
			</button>
			<p id="e2e-forced-failure-result">{forcedFailureResult()}</p>
			<button
				id="e2e-run-slow-increment-race-button"
				type="button"
				onClick={runSlowIncrementRace}
			>
				Run Slow Increment Race
			</button>
			<p id="e2e-slow-increment-race-result">
				{slowIncrementRaceResult()}
			</p>

			<div id="e2e-stress-lab">
				<button
					id="e2e-run-stress-button"
					type="button"
					onClick={runStressMatrix}
				>
					Run Deterministic Stress Matrix
				</button>
				<p id="e2e-stress-summary">{stressSummary()}</p>
				<p id="e2e-stress-total-count">{allStressCases.length}</p>
				<p id="e2e-stress-failure-count">{stressFailureCount()}</p>

				<table id="e2e-stress-results-table">
					<thead>
						<tr>
							<th>Case</th>
							<th>Status</th>
						</tr>
					</thead>
					<tbody>
						<For each={stressRows()}>
							{(row) => (
								<tr data-case-label={row.caseLabel}>
									<td>{row.caseLabel}</td>
									<td>{deriveRowStatusText(row)}</td>
								</tr>
							)}
						</For>
					</tbody>
				</table>
			</div>

			<form
				id="e2e-redirect-form"
				method="post"
				action={redirectActionURL.toString()}
			>
				<input
					id="e2e-redirect-target"
					name="target"
					value="/users/redirected-from-action?q=redirected"
				/>
				<button id="e2e-redirect-submit" type="submit">
					Submit Redirect Action
				</button>
			</form>
		</section>
	);
}

function deriveRowPasses(row: StressResultRow): boolean {
	return (
		row.simpleEchoMatched && row.complexEchoMatched && row.scoreTotalMatched
	);
}

function deriveRowStatusText(row: StressResultRow): string {
	if (deriveRowPasses(row)) {
		return "pass";
	}
	return [
		!row.simpleEchoMatched ? "simple-echo-mismatch" : "",
		!row.complexEchoMatched ? "complex-echo-mismatch" : "",
		!row.scoreTotalMatched ? "score-total-mismatch" : "",
	]
		.filter((reason) => reason !== "")
		.join("|");
}

function buildComplexEchoPayloadForStressCase(input: {
	stressCase: EchoStressCase;
}): ComplexEchoPayload {
	return {
		title: `${input.stressCase.caseLabel}:${input.stressCase.value}`,
		flags: [
			input.stressCase.amount >= 0,
			input.stressCase.value.length > 12,
		],
		scores: [
			input.stressCase.amount,
			input.stressCase.value.length,
			input.stressCase.amount * -1,
		],
		metadata: {
			caseLabel: input.stressCase.caseLabel,
			valueLength: String(input.stressCase.value.length),
		},
	};
}

function buildDeterministicStressCases(input: {
	seed: number;
	caseCount: number;
}): EchoStressCase[] {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_=+[]{}()!?/";
	let state = input.seed >>> 0;
	const generatedCases: EchoStressCase[] = [];

	for (let index = 0; index < input.caseCount; index += 1) {
		state = nextDeterministicState({ state });
		const valueLength = (state % 30) + 1;
		let value = "";

		for (let charIndex = 0; charIndex < valueLength; charIndex += 1) {
			state = nextDeterministicState({ state });
			const alphabetIndex = state % alphabet.length;
			value += alphabet[alphabetIndex] ?? "x";
		}

		state = nextDeterministicState({ state });
		const signedAmount = Number(state % 8000) - 4000;
		generatedCases.push({
			caseLabel: `generated-${index + 1}`,
			value,
			amount: signedAmount,
		});
	}

	return generatedCases;
}

function nextDeterministicState(input: { state: number }): number {
	return (input.state * 1_664_525 + 1_013_904_223) >>> 0;
}

async function sleepMS(milliseconds: number): Promise<void> {
	await new Promise((resolve) => {
		setTimeout(resolve, milliseconds);
	});
}
