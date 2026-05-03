export interface ConfigEditorOption {
  label: string;
  value: string;
}

interface ConfigEditorControlBase {
  id: string;
  label: string;
  description?: string;
}

export interface TemplateRender {
  mode: "template";
  lines: string[];
  separator?: string;
}

export interface ToggleRender {
  mode: "toggle";
  trueLines: string[];
  falseLines?: string[];
}

export interface ChoiceRender {
  mode: "choice";
  choices: Record<string, string[]>;
}

export interface JoinRender {
  mode: "join";
  lines: string[];
  separator?: string;
}

export interface SequenceRender {
  mode: "sequence";
  beforeLines?: string[];
  itemLines: string[];
  afterLines?: string[];
  emptyLines?: string[];
  separator?: string;
}

export type ConfigEditorRender =
  | TemplateRender
  | ToggleRender
  | ChoiceRender
  | JoinRender
  | SequenceRender;

export interface ConfigEditorCheckboxControl extends ConfigEditorControlBase {
  type: "checkbox";
  defaultValue: boolean;
  render: ToggleRender;
}

export interface ConfigEditorRadioControl extends ConfigEditorControlBase {
  type: "radio";
  defaultValue: string;
  options: ConfigEditorOption[];
  render: ChoiceRender;
}

export interface ConfigEditorNumberControl extends ConfigEditorControlBase {
  type: "number";
  defaultValue: number;
  min?: number;
  max?: number;
  step?: number;
  render: TemplateRender;
}

export interface ConfigEditorTextControl extends ConfigEditorControlBase {
  type: "text";
  defaultValue: string;
  placeholder?: string;
  render: TemplateRender;
}

export interface ConfigEditorMultiselectControl extends ConfigEditorControlBase {
  type: "multiselect";
  defaultValue: string[];
  options: ConfigEditorOption[];
  render: JoinRender | SequenceRender;
}

export type ConfigEditorControl =
  | ConfigEditorCheckboxControl
  | ConfigEditorRadioControl
  | ConfigEditorNumberControl
  | ConfigEditorTextControl
  | ConfigEditorMultiselectControl;

export interface ConfigEditorDefinition {
  managedBlockStart: string;
  managedBlockEnd: string;
  controls: ConfigEditorControl[];
}

export interface ConfigEditorSection {
  controlId: string;
  text: string;
}

export type ConfigEditorValue = boolean | number | string | string[];
export type ConfigEditorState = Record<string, ConfigEditorValue>;

export function buildInitialConfigEditorState(editor: ConfigEditorDefinition): ConfigEditorState {
  return Object.fromEntries(editor.controls.map(control => [control.id, control.defaultValue]));
}

function interpolate(template: string, context: Record<string, string | number | boolean>): string {
  return template.replace(/\{\{\s*([a-zA-Z0-9_]+)\s*\}\}/g, (_, key: string) => {
    const value = context[key];
    return value === undefined ? "" : String(value);
  });
}

function renderLines(lines: string[] | undefined, context: Record<string, string | number | boolean>): string[] {
  return (lines ?? []).map(line => interpolate(line, context));
}

function renderControl(control: ConfigEditorControl, rawValue: ConfigEditorValue): string[] {
  switch (control.type) {
    case "checkbox": {
      const value = Boolean(rawValue);
      return value ? control.render.trueLines : (control.render.falseLines ?? []);
    }
    case "radio": {
      const value = String(rawValue);
      return control.render.choices[value] ?? [];
    }
    case "number":
    case "text": {
      const value = control.type === "number" ? Number(rawValue) : String(rawValue);
      return renderLines(control.render.lines, { value });
    }
    case "multiselect": {
      const values = Array.isArray(rawValue) ? rawValue.map(String) : [];
      if (control.render.mode === "join") {
        const joined = values.join(control.render.separator ?? " ");
        return renderLines(control.render.lines, {
          joined,
          count: values.length,
          first: values[0] ?? "",
          last: values[values.length - 1] ?? "",
        });
      }
      if (values.length === 0) {
        return renderLines(control.render.emptyLines, { count: 0 });
      }
      const lines = renderLines(control.render.beforeLines, {
        count: values.length,
        first: values[0] ?? "",
        second: values[1] ?? values[0] ?? "",
        secondIndex1: values.length > 1 ? 2 : 1,
        last: values[values.length - 1] ?? "",
      });
      for (let index = 0; index < values.length; index += 1) {
        const value = values[index] ?? "";
        const nextValue = values[(index + 1) % values.length] ?? "";
        lines.push(
          ...renderLines(control.render.itemLines, {
            value,
            nextValue,
            index,
            index1: index + 1,
            nextIndex: (index + 1) % values.length,
            nextIndex1: ((index + 1) % values.length) + 1,
            count: values.length,
            first: values[0] ?? "",
            last: values[values.length - 1] ?? "",
          })
        );
      }
      lines.push(
        ...renderLines(control.render.afterLines, {
          count: values.length,
          first: values[0] ?? "",
          second: values[1] ?? values[0] ?? "",
          secondIndex1: values.length > 1 ? 2 : 1,
          last: values[values.length - 1] ?? "",
        })
      );
      return lines;
    }
  }
}

export function buildManagedSections(editor: ConfigEditorDefinition, state: ConfigEditorState): ConfigEditorSection[] {
  return editor.controls.map(control => ({
    controlId: control.id,
    text: renderControl(control, state[control.id]).join("\n"),
  }));
}

export function buildManagedConfig(editor: ConfigEditorDefinition, state: ConfigEditorState): string {
  return buildManagedSections(editor, state).map(section => section.text).join("\n");
}

function normalizeConfigText(text: string): string {
  return text.replace(/\r\n/g, "\n");
}

export function applyManagedConfig(
  configText: string,
  editor: ConfigEditorDefinition,
  state: ConfigEditorState
): string {
  const normalized = normalizeConfigText(configText);
  const managed = buildManagedConfig(editor, state);
  const block = `${editor.managedBlockStart}\n${managed}\n${editor.managedBlockEnd}`;
  const start = normalized.indexOf(editor.managedBlockStart);
  const end = normalized.indexOf(editor.managedBlockEnd);
  if (start >= 0 && end >= start) {
    const afterEnd = end + editor.managedBlockEnd.length;
    return `${normalized.slice(0, start)}${block}${normalized.slice(afterEnd)}`;
  }
  const separator = normalized.endsWith("\n") ? "" : "\n";
  return `${normalized}${separator}\n${block}\n`;
}

export function findControlSelection(
  configText: string,
  editor: ConfigEditorDefinition,
  state: ConfigEditorState,
  controlId: string
): { start: number; end: number } | null {
  const normalized = normalizeConfigText(configText);
  const managedStart = normalized.indexOf(editor.managedBlockStart);
  if (managedStart < 0) return null;

  const sections = buildManagedSections(editor, state);
  let offset = managedStart + editor.managedBlockStart.length + 1;

  for (const section of sections) {
    const start = offset;
    const end = start + section.text.length;
    if (section.controlId === controlId) {
      return { start, end };
    }
    offset = end + 1;
  }

  return null;
}
