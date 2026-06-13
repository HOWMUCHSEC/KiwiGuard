import assert from "node:assert/strict";
import { readdir, readFile, stat } from "node:fs/promises";

const sourceRoot = new URL("../src/", import.meta.url);
const sourceFiles = await listSourceFiles(sourceRoot);

for (const file of sourceFiles) {
  const source = await readFile(file, "utf8");
  const relativePath = relativeSourcePath(file);
  for (const comment of collectComments(source)) {
    assert.doesNotMatch(
      comment.text,
      /\p{Script=Han}/u,
      `${relativePath}:${comment.line} contains non-English source comment text; keep user-facing Chinese in i18n resources`
    );
  }
}

console.log("source comment checks passed");

async function listSourceFiles(dir) {
  const entries = await readdir(dir);
  const files = [];
  for (const entry of entries) {
    const child = new URL(entry, dir);
    const info = await stat(child);
    if (info.isDirectory()) {
      files.push(...(await listSourceFiles(new URL(`${child.href}/`))));
    } else if (/\.(css|ts|tsx)$/.test(entry)) {
      files.push(child);
    }
  }
  return files;
}

function collectComments(source) {
  const comments = [];
  let index = 0;
  let line = 1;

  while (index < source.length) {
    const char = source[index];
    const next = source[index + 1];

    if (char === "\n") {
      line += 1;
      index += 1;
      continue;
    }

    if (char === '"' || char === "'" || char === "`") {
      const result = skipString(source, index, line, char);
      index = result.index;
      line = result.line;
      continue;
    }

    if (char === "/" && next === "/") {
      const startLine = line;
      const end = source.indexOf("\n", index + 2);
      const commentEnd = end === -1 ? source.length : end;
      comments.push({ line: startLine, text: source.slice(index, commentEnd) });
      index = commentEnd;
      continue;
    }

    if (char === "/" && next === "*") {
      const startLine = line;
      const end = source.indexOf("*/", index + 2);
      const commentEnd = end === -1 ? source.length : end + 2;
      const text = source.slice(index, commentEnd);
      comments.push({ line: startLine, text });
      line += countNewlines(text);
      index = commentEnd;
      continue;
    }

    index += 1;
  }

  return comments;
}

function skipString(source, start, line, quote) {
  let index = start + 1;
  while (index < source.length) {
    const char = source[index];
    if (char === "\n") line += 1;
    if (char === "\\") {
      index += 2;
      continue;
    }
    if (char === quote) {
      return { index: index + 1, line };
    }
    index += 1;
  }
  return { index, line };
}

function countNewlines(value) {
  return value.split("\n").length - 1;
}

function relativeSourcePath(file) {
  const [, relative] = file.pathname.split("/web/src/");
  return relative;
}
