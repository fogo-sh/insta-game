// The existing game.json editor definitions remain the source of engine-specific settings.
export function settings(game, container, textarea) {
  container.replaceChildren();
  const editor = game.configEditor;
  if (!editor) return;
  const values = Object.fromEntries(
    editor.controls.map((c) => [c.id, c.defaultValue]),
  );
  const title = document.createElement("h3");
  title.className = "settings-title";
  title.textContent = "Game settings";
  container.append(title);
  function apply() {
    const lines = editor.controls.flatMap((c) => {
      const value = values[c.id],
        render = c.render;
      if (render.mode === "toggle")
        return value ? render.trueLines : render.falseLines;
      if (render.mode === "choice") return render.choices[value] || [];
      const safe = (v) =>
        String(v)
          .replaceAll("\\", "\\\\")
          .replaceAll('"', '\\"')
          .replaceAll("\n", " ")
          .replaceAll("\r", " ");
      return render.lines.map((line) =>
        line
          .replaceAll("{{value}}", safe(value))
          .replaceAll(
            "{{joined}}",
            (Array.isArray(value) ? value : [])
              .map(safe)
              .join(render.separator || " "),
          ),
      );
    });
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
