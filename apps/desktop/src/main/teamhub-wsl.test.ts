import { execFileSync, spawnSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect, it } from "vitest";

const wslAvailable =
  process.platform === "win32" &&
  spawnSync("wsl.exe", ["--exec", "/bin/true"], {
    timeout: 15_000,
    windowsHide: true,
  }).status === 0;

it.skipIf(!wslAvailable)(
  "runs the Desktop TeamHub script through WSL with Windows paths and environment",
  () => {
    const root = mkdtempSync(path.join(tmpdir(), "linger teamhub wsl "));
    try {
      const cli = path.join(root, "resources", "app", "dist", "teamhub-cli");
      mkdirSync(cli, { recursive: true });
      copyFileSync(process.execPath, path.join(root, "Linger.exe"));
      writeFileSync(
        path.join(cli, "teamhub"),
        readFileSync(path.resolve("scripts/teamhub.sh"), "utf8").replace(
          /\r\n/g,
          "\n",
        ),
      );
      writeFileSync(
        path.join(cli, "teamhub.js"),
        "process.stdout.write(JSON.stringify({node:process.env.ELECTRON_RUN_AS_NODE,arg:process.argv[2]}))",
      );
      const unixPath = execFileSync(
        "wsl.exe",
        ["--exec", "wslpath", "-u", path.join(cli, "teamhub")],
        {
          encoding: "utf8",
          timeout: 15_000,
          windowsHide: true,
        },
      ).trim();
      const result = execFileSync(
        "wsl.exe",
        ["--exec", "/bin/sh", unixPath, "argument with spaces"],
        {
          encoding: "utf8",
          timeout: 30_000,
          windowsHide: true,
          env: { ...process.env, ELECTRON_RUN_AS_NODE: "" },
        },
      );
      expect(JSON.parse(result)).toEqual({
        node: "1",
        arg: "argument with spaces",
      });
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  },
  60_000,
);
