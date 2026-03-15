import { useState, useEffect } from "react";
import { TextAttributes } from "@opentui/core";

const BRAILLE = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"] as const;

export function Spinner({ message }: { message: string }) {
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    const id = setInterval(() => setFrame((f: number) => (f + 1) % BRAILLE.length), 80);
    return () => clearInterval(id);
  }, []);

  return (
    <text>
      <span fg="#00bcd4" attributes={TextAttributes.BOLD}>{BRAILLE[frame]}</span>
      {" "}{message}
    </text>
  );
}
