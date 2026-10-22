function renderLines(lines = [], values) {
  const safe = (value) =>
    String(value)
      .replaceAll("\\", "\\\\")
      .replaceAll('"', '\\"')
      .replaceAll("\n", " ")
      .replaceAll("\r", " ");
  return lines.map((line) =>
    line.replace(/\{\{(\w+)\}\}/g, (_, key) => safe(values[key] ?? "")),
  );
}

export function renderControl(render, value) {
  if (render.mode === "toggle")
    return value ? render.trueLines : render.falseLines;
  if (render.mode === "choice") return render.choices[value] || [];
  if (render.mode === "sequence") {
    if (!value.length) return renderLines(render.emptyLines, { count: 0 });
    const common = {
      count: value.length,
      first: value[0],
      second: value[1] ?? value[0],
      secondIndex1: value.length > 1 ? 2 : 1,
      last: value[value.length - 1],
    };
    return [
      ...renderLines(render.beforeLines, common),
      ...value.flatMap((item, index) => {
        const nextIndex = (index + 1) % value.length;
        return renderLines(render.itemLines, {
          ...common,
          value: item,
          nextValue: value[nextIndex],
          index,
          index1: index + 1,
          nextIndex,
          nextIndex1: nextIndex + 1,
        });
      }),
      ...renderLines(render.afterLines, common),
    ];
  }
  return renderLines(render.lines, {
    value,
    joined: Array.isArray(value) ? value.join(render.separator || " ") : "",
  });
}

export function settings(game, container, textarea) {
  container.replaceChildren();
  const editor = game.configEditor;
  if (!editor) return;
  const values = Object.fromEntries(
    editor.controls.map((c) => [c.id, c.defaultValue]),
  );
  function apply() {
    const lines = editor.controls.flatMap((c) =>
      renderControl(c.render, values[c.id]),
    );
    const start = textarea.value.indexOf(editor.managedBlockStart);
    const end = textarea.value.indexOf(editor.managedBlockEnd);
    const block =
      editor.managedBlockStart +
      "\n" +
      lines.join("\n") +
      "\n" +
      editor.managedBlockEnd;
    textarea.value =
      start >= 0 && end >= start
        ? textarea.value.slice(0, start) +
          block +
          textarea.value.slice(end + editor.managedBlockEnd.length)
        : textarea.value + "\n" + block + "\n";
  }
  for (const control of editor.controls) {
    const label = document.createElement("label");
    label.textContent = control.label;
    let input;
    if (control.type === "radio" || control.type === "multiselect") {
      input = document.createElement("select");
      input.multiple = control.type === "multiselect";
      for (const option of control.options) {
        const item = document.createElement("option");
        item.value = option.value;
        item.textContent = option.label;
        item.selected = input.multiple
          ? values[control.id].includes(option.value)
          : values[control.id] === option.value;
        input.append(item);
      }
    } else {
      input = document.createElement("input");
      input.type =
        control.type === "checkbox"
          ? "checkbox"
          : control.type === "number"
            ? "number"
            : "text";
      if (input.type === "checkbox") {
        input.checked = Boolean(values[control.id]);
        label.className = "check";
      } else input.value = values[control.id];
      if (control.min !== undefined) input.min = control.min;
      if (control.max !== undefined) input.max = control.max;
    }
    input.addEventListener("change", () => {
      values[control.id] =
        input.type === "checkbox"
          ? input.checked
          : input.multiple
            ? [...input.selectedOptions].map((o) => o.value)
            : input.type === "number"
              ? Number(input.value)
              : input.value;
      apply();
    });
    if (input.type === "checkbox") label.prepend(input);
    else label.append(input);
    container.append(label);
  }
}
