import { TextAttributes } from "@opentui/core";

export function App() {
  return (
    <box alignItems="center" justifyContent="center" flexGrow={1}>
      <box justifyContent="center" alignItems="flex-end">
        <ascii-font font="tiny" text="fz" />
        <text attributes={TextAttributes.DIM}>ForgeZ — agentic workflow CLI</text>
      </box>
    </box>
  );
}
