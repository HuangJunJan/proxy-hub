import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { Button } from "./button";

export function CopyButton({
  copiedLabel,
  label,
  value,
}: {
  copiedLabel: string;
  label: string;
  value: string;
}) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    await writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  return (
    <Button aria-label={copied ? copiedLabel : label} onClick={() => void copy()} size="sm" type="button" variant="outline">
      {copied ? <Check size={15} /> : <Copy size={15} />}
      {copied ? copiedLabel : label}
    </Button>
  );
}

async function writeText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.left = "-9999px";
  textarea.style.position = "fixed";
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand("copy");
  document.body.removeChild(textarea);
}
