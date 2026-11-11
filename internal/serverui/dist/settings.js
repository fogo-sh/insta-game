// src/settings.ts
function renderLines(lines = [], values) {
	const safe = (value) => String(value).replaceAll("\\", "\\\\").replaceAll("\"", "\\\"").replaceAll("\n", " ").replaceAll("\r", " ");
	return lines.map((line) => line.replace(/\{\{(\w+)\}\}/g, (_, key) => safe(values[key] ?? "")));
}
function renderControl(render, value) {
	if (render.mode === "toggle") return value ? render.trueLines : render.falseLines;
	if (render.mode === "choice") return render.choices[String(value)] || [];
	if (render.mode === "sequence") {
		if (!Array.isArray(value)) throw new Error("Sequence controls require a list of values.");
		const first = value[0];
		if (first === void 0) return renderLines(render.emptyLines, { count: 0 });
		const common = {
			count: value.length,
			first,
			second: value[1] ?? first,
			secondIndex1: value.length > 1 ? 2 : 1,
			last: value[value.length - 1] ?? first
		};
		return [
			...renderLines(render.beforeLines, common),
			...value.flatMap((item, index) => {
				const nextIndex = (index + 1) % value.length;
				return renderLines(render.itemLines, {
					...common,
					value: item,
					nextValue: value[nextIndex] ?? first,
					index,
					index1: index + 1,
					nextIndex,
					nextIndex1: nextIndex + 1
				});
			}),
			...renderLines(render.afterLines, common)
		];
	}
	return renderLines(render.lines, {
		value,
		joined: Array.isArray(value) ? value.join(render.separator || " ") : ""
	});
}
function settings(game, container, textarea) {
	container.replaceChildren();
	if (!game.configEditor) return;
	const editor = game.configEditor;
	const values = Object.fromEntries(editor.controls.map((c) => [c.id, c.defaultValue]));
	function apply() {
		const lines = editor.controls.flatMap((c) => renderControl(c.render, values[c.id] ?? c.defaultValue));
		const start = textarea.value.indexOf(editor.managedBlockStart);
		const end = textarea.value.indexOf(editor.managedBlockEnd);
		const block = editor.managedBlockStart + "\n" + lines.join("\n") + "\n" + editor.managedBlockEnd;
		textarea.value = start >= 0 && end >= start ? textarea.value.slice(0, start) + block + textarea.value.slice(end + editor.managedBlockEnd.length) : textarea.value + "\n" + block + "\n";
	}
	for (const control of editor.controls) {
		const label = document.createElement("label");
		label.textContent = control.label;
		let input;
		if (control.type === "radio" || control.type === "multiselect") {
			input = document.createElement("select");
			input.multiple = control.type === "multiselect";
			const current = values[control.id];
			for (const option of control.options) {
				const item = document.createElement("option");
				item.value = option.value;
				item.textContent = option.label;
				item.selected = input.multiple ? Array.isArray(current) && current.includes(option.value) : current === option.value;
				input.append(item);
			}
		} else {
			input = document.createElement("input");
			input.type = control.type === "checkbox" ? "checkbox" : control.type === "number" ? "number" : "text";
			if (input.type === "checkbox") {
				input.checked = Boolean(values[control.id]);
				label.className = "check";
			} else input.value = String(values[control.id]);
			if (control.type === "number") {
				if (control.min !== void 0) input.min = String(control.min);
				if (control.max !== void 0) input.max = String(control.max);
			}
		}
		input.setAttribute("aria-label", control.label);
		input.addEventListener("change", () => {
			values[control.id] = input instanceof HTMLInputElement && input.type === "checkbox" ? input.checked : input instanceof HTMLSelectElement && input.multiple ? [...input.selectedOptions].map((o) => o.value) : input.type === "number" ? Number(input.value) : input.value;
			apply();
		});
		if (input.type === "checkbox") label.prepend(input);
		else label.append(input);
		container.append(label);
	}
}
export { renderControl, settings };
