// ==UserScript==
// @name         Komodo confirmation text copier
// @namespace    clip-share-tools
// @version      1.0.0
// @description  Click Komodo's "Please enter … below" text to copy the confirmation value.
// @match        http://192.168.1.66:30160/*
// @grant        GM_setClipboard
// @run-at       document-idle
// ==/UserScript==

(function () {
  "use strict";

  const marker = "Please enter";
  const handled = "data-clip-share-copy-ready";

  function copyConfirmationValue(prompt) {
    const value = prompt.querySelector("b")?.textContent?.trim();
    if (!value) return;

    GM_setClipboard(value, "text");
    const original = prompt.textContent;
    prompt.textContent = `Copied “${value}” to clipboard`;
    prompt.style.color = "var(--mantine-color-green-6, #40c057)";
    window.setTimeout(() => {
      prompt.textContent = original;
      prompt.style.color = "";
    }, 1400);
  }

  function enhance() {
    for (const paragraph of document.querySelectorAll("p")) {
      if (paragraph.getAttribute(handled) || !paragraph.textContent?.includes(marker) || !paragraph.querySelector("b")) continue;
      paragraph.setAttribute(handled, "true");
      paragraph.title = "Click to copy the confirmation text";
      paragraph.style.cursor = "pointer";
      paragraph.addEventListener("click", () => copyConfirmationValue(paragraph));
    }
  }

  enhance();
  new MutationObserver(enhance).observe(document.body, { childList: true, subtree: true });
})();
