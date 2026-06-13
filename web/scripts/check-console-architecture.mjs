import assert from "node:assert/strict";
import { access, readdir, readFile, stat } from "node:fs/promises";
import path from "node:path/posix";
import ts from "typescript";

const sourceFiles = await listSourceFiles(new URL("../src", import.meta.url));
const modules = await loadModules(sourceFiles);
const moduleByPath = new Map(modules.map((module) => [module.relativePath, module]));

await assertPathExists("../src/app/App.tsx");
await assertPathExists("../src/app/providers/AppProviders.tsx");
await assertPathExists("../src/app/router/useConsoleRouter.ts");
await assertPathExists("../src/app/shell/ConsoleHeader.tsx");
await assertPathExists("../src/app/shell/ConsoleShell.tsx");
await assertPathExists("../src/app/shell/ConsoleSummary.tsx");
await assertPathExists("../src/console/manifest.ts");
await assertPathExists("../src/console/metrics.ts");
await assertPathExists("../src/console/routes.ts");
await assertPathExists("../src/features/console-summary/public.ts");
await assertPathExists("../src/features/console-summary/model/useConsoleSummary.ts");
await assertPathExists("../src/platform/api/consoleSummary.ts");
await assertPathExists("../src/platform/api/http.ts");
await assertPathExists("../src/platform/i18n/types.ts");
await assertPathExists("../src/platform/query/keys.ts");
await assertPathExists("../src/shared/types/console.ts");
await assertPathExists("../src/shared/types/spool.ts");
await assertPathExists("../src/shared/types/traffic.ts");
await assertPathExists("../src/shared/utils/README.md");
await assertPathExists("../src/shared/ui/surfaces.css");
await assertPathMissing("../src/app/useConsoleSummaryData.ts", "app/useConsoleSummaryData.ts must not exist; global summary belongs to features/console-summary");
await assertPathMissing("../src/console/navigation.ts", "console/navigation.ts must not exist; use console/routes directly");
await assertPathMissing("../src/platform/api/consolePosture.ts", "console posture view models belong in shared/types, not platform/api");
await assertPathMissing("../src/shared/api", "shared/api must not exist; infrastructure API clients belong in platform/api");
await assertPathMissing("../src/shared/i18n", "shared/i18n must not exist; i18n belongs in platform/i18n");

for (const domain of ["dashboard", "traffic", "policies", "routing", "storage"]) {
  await assertPathExists(`../src/features/${domain}/public.ts`);
}

const appSource = moduleByPath.get("app/App.tsx")?.source ?? "";
assert.match(appSource, /useConsoleRouter/, "App.tsx must consume the app/router adapter");
assert.match(appSource, /useConsolePostureSummary/, "App.tsx must consume the console-summary feature contract");
assert.doesNotMatch(appSource, /window\.location|location\.hash/, "App.tsx must not read or write the hash directly");
assert.doesNotMatch(appSource, /features\/(?:dashboard|traffic|policies|routing|storage)\/(?:page|model|ui)\//, "App.tsx must not import feature internals");

const summarySource = moduleByPath.get("app/shell/ConsoleSummary.tsx")?.source ?? "";
assert.match(summarySource, /consoleMetricDefinitions/, "ConsoleSummary must render consoleMetricDefinitions");
assert.doesNotMatch(summarySource, /\bif\s*\(|switch\s*\(/, "ConsoleSummary must not contain metric-specific branching");

const summaryModelSource = moduleByPath.get("features/console-summary/model/useConsoleSummary.ts")?.source ?? "";
assert.match(summaryModelSource, /getConsoleSummary/, "console-summary feature must use the lightweight backend summary endpoint");
for (const module of modules.filter((entry) => entry.relativePath.startsWith("features/console-summary/"))) {
  assert.doesNotMatch(module.source, /listTrafficEvents|traffic-events|\/api\/traffic\/events/, `${module.relativePath} must not derive global metrics from traffic event lists`);
}

for (const module of modules) {
  assertAliasImports(module);
  assertImportBoundaries(module);
  assertNoLegacyRelativeImports(module);
  assertNoRawQueryKeys(module);
  assertPublicBoundary(module);
}

const surfacesSource = await readFile(new URL("../src/shared/ui/surfaces.css", import.meta.url), "utf8");
assert.doesNotMatch(surfacesSource, /#[0-9a-fA-F]{3,8}\b/, "shared/ui/surfaces.css should consume semantic theme variables instead of raw hex colors");

console.log("console architecture checks passed");

function assertAliasImports(module) {
  assert.doesNotMatch(module.source, /from\s+["'][^"']*shared\/api\//, `${module.relativePath} must use platform/api instead of shared/api`);
  assert.doesNotMatch(module.source, /from\s+["'][^"']*shared\/i18n/, `${module.relativePath} must use platform/i18n instead of shared/i18n`);
}

function assertImportBoundaries(module) {
  const origin = classifyModule(module.relativePath);
  for (const dependency of collectModuleSpecifiers(module.sourceFile)) {
    const targetPath = resolveImport(module.relativePath, dependency.specifier);
    if (!targetPath) continue;
    const target = classifyModule(targetPath);
    const error = importBoundaryError(origin, module.relativePath, target, targetPath);
    assert.equal(error, null, error ?? undefined);
  }
}

function assertNoLegacyRelativeImports(module) {
  if (!module.relativePath.startsWith("app/")) return;
  assert.doesNotMatch(module.source, /from\s+["']\.\.\/features\//, `${module.relativePath} must use feature public aliases, not relative feature imports`);
}

function assertNoRawQueryKeys(module) {
  assert.doesNotMatch(module.source, /queryKey:\s*\[/, `${module.relativePath} must use platform/query queryKeys instead of raw query key arrays`);
  assert.doesNotMatch(module.source, /invalidateQueries\(\{\s*queryKey:\s*\[/, `${module.relativePath} must invalidate via platform/query queryKeys`);
}

function assertPublicBoundary(module) {
  if (!/^features\/[^/]+\/public\.ts$/.test(module.relativePath)) return;

  for (const statement of module.sourceFile.statements) {
    if (!ts.isExportDeclaration(statement) || !statement.exportClause || !ts.isNamedExports(statement.exportClause)) continue;

    for (const element of statement.exportClause.elements) {
      const exportedName = element.name.text;
      const isAllowed =
        exportedName.endsWith("Page") ||
        exportedName.startsWith("use") ||
        /^[A-Z]/.test(exportedName);
      assert.equal(
        isAllowed,
        true,
        `${module.relativePath} exports low-level capability '${exportedName}'; move API clients/use cases to platform/api or keep them feature-internal`
      );
    }
  }
}

function importBoundaryError(origin, originPath, target, targetPath) {
  if (origin.layer === "shared" && target.layer !== "shared") {
    return `${originPath} must stay domain-agnostic; found import of ${targetPath}`;
  }

  if (origin.layer === "platform" && !["platform", "shared"].includes(target.layer)) {
    return `${originPath} may import only platform/shared modules; found ${targetPath}`;
  }

  if (origin.layer === "console" && !["console", "shared", "platform"].includes(target.layer)) {
    return `${originPath} may import only console/platform/shared modules; found ${targetPath}`;
  }

  if (origin.layer === "app" && target.layer === "feature" && !targetPath.endsWith("/public.ts")) {
    return `${originPath} must consume feature capabilities through features/*/public.ts; found ${targetPath}`;
  }

  if (origin.layer === "feature" && target.layer === "app") {
    return `${originPath} must not depend on app-layer modules; found ${targetPath}`;
  }

  if (origin.layer === "feature" && target.layer === "console") {
    return `${originPath} must not depend on console metadata; adapt console routes at app boundaries`;
  }

  if (origin.layer === "feature" && originPath.includes("/page/") && targetPath === "platform/api/http.ts") {
    return `${originPath} must not import the low-level HTTP client; use a feature model API client`;
  }

  if (origin.layer === "feature" && target.layer === "feature" && origin.domain !== target.domain) {
    if (targetPath.endsWith("/public.ts")) return null;
    return `${originPath} crosses feature boundaries through ${targetPath}; use public.ts or move the contract to platform/shared`;
  }

  return null;
}

function collectModuleSpecifiers(sourceFile) {
  const specifiers = [];
  for (const statement of sourceFile.statements) {
    if ((ts.isImportDeclaration(statement) || ts.isExportDeclaration(statement)) && statement.moduleSpecifier && ts.isStringLiteral(statement.moduleSpecifier)) {
      specifiers.push({ specifier: statement.moduleSpecifier.text });
    }
  }
  return specifiers;
}

function resolveImport(fromPath, specifier) {
  if (specifier.startsWith(".")) return resolveRelativeImport(fromPath, specifier);
  for (const prefix of ["app", "console", "features", "platform", "shared"]) {
    if (specifier === prefix || specifier.startsWith(`${prefix}/`)) return resolveCandidate(specifier);
  }
  return null;
}

function resolveRelativeImport(fromPath, specifier) {
  return resolveCandidate(path.normalize(path.join(path.dirname(fromPath), specifier)));
}

function resolveCandidate(basePath) {
  const candidates = [
    basePath,
    `${basePath}.ts`,
    `${basePath}.tsx`,
    `${basePath}/index.ts`,
    `${basePath}/index.tsx`
  ];
  return candidates.find((candidate) => moduleByPath.has(candidate)) ?? null;
}

function classifyModule(relativePath) {
  if (relativePath === "main.tsx") return { layer: "root" };
  if (relativePath.startsWith("app/")) return { layer: "app" };
  if (relativePath.startsWith("console/")) return { layer: "console" };
  if (relativePath.startsWith("features/")) {
    const [, domain] = relativePath.split("/");
    return { layer: "feature", domain };
  }
  if (relativePath.startsWith("platform/")) return { layer: "platform" };
  if (relativePath.startsWith("shared/")) return { layer: "shared" };
  return { layer: "unknown" };
}

async function assertPathExists(filePath) {
  await access(new URL(filePath, import.meta.url));
}

async function assertPathMissing(filePath, message) {
  try {
    await access(new URL(filePath, import.meta.url));
    assert.fail(message);
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
}

async function listSourceFiles(dir) {
  const directory = dir.href.endsWith("/") ? dir : new URL(`${dir.href}/`);
  const entries = await readdir(directory);
  const files = [];
  for (const entry of entries) {
    const child = new URL(entry, directory);
    const info = await stat(child);
    if (info.isDirectory()) {
      files.push(...(await listSourceFiles(new URL(`${child.href}/`))));
    } else if (/\.(ts|tsx)$/.test(entry)) {
      files.push(child);
    }
  }
  return files;
}

async function loadModules(files) {
  return Promise.all(files.map(async (file) => {
    const source = await readFile(file, "utf8");
    const relativePath = relativeSourcePath(file);
    return {
      file,
      relativePath,
      source,
      sourceFile: ts.createSourceFile(relativePath, source, ts.ScriptTarget.Latest, true, file.pathname.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
    };
  }));
}

function relativeSourcePath(file) {
  const [, relative] = file.pathname.split("/web/src/");
  return relative;
}
