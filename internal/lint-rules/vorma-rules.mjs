export default {
	meta: { name: "vorma-rules" },
	rules: {
		"smart-explicit-returns": {
			meta: {
				type: "suggestion",
				docs: {
					description:
						"Require braces and explicit return when an arrow function's concise body is an object literal or spans multiple lines.",
				},
				fixable: "code",
				schema: [],
				messages: {
					useExplicitReturn:
						"Arrow functions returning object literals or with multiline concise bodies should use braces and an explicit return statement.",
				},
			},
			create(context) {
				const source = context.sourceCode ?? context.getSourceCode();
				return {
					ArrowFunctionExpression(node) {
						if (!node.expression) {
							return;
						}
						const isObject = node.body.type === "ObjectExpression";
						const isMultiline =
							node.body.loc.start.line !== node.body.loc.end.line;
						if (!isObject && !isMultiline) {
							return;
						}
						context.report({
							node: node.body,
							messageId: "useExplicitReturn",
							fix(fixer) {
								const bodyText = source.getText(node.body);
								const tokenBefore = source.getTokenBefore(
									node.body,
								);
								const tokenAfter = source.getTokenAfter(
									node.body,
								);
								if (
									tokenBefore &&
									tokenBefore.value === "(" &&
									tokenAfter &&
									tokenAfter.value === ")"
								) {
									return fixer.replaceTextRange(
										[
											tokenBefore.range[0],
											tokenAfter.range[1],
										],
										`{ return ${bodyText}; }`,
									);
								}
								return fixer.replaceText(
									node.body,
									`{ return ${bodyText}; }`,
								);
							},
						});
					},
				};
			},
		},
	},
};
