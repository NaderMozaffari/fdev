'use strict';

// "fdev: Open": runs the fdev launcher in a terminal in the editor area,
// in the workspace folder of the file being edited. The terminal is named
// "fdev"; opening again focuses it instead of starting a second one.
//
// FDEV_FROM_VSCODE=1 is put in the terminal's environment so that a script
// (or the program) can tell it was started from the extension; fdev itself
// already knows it is in the editor from TERM_PROGRAM=vscode.

const fs = require('fs');
const os = require('os');
const path = require('path');
const childProcess = require('child_process');
const vscode = require('vscode');

const TERMINAL_NAME = 'fdev';
const INSTALL = 'go install github.com/NaderMozaffari/fdev@latest';
const isWindows = process.platform === 'win32';

function isExecutable(file) {
  try {
    const stat = fs.statSync(file);
    if (!stat.isFile()) return false;
    if (!isWindows) fs.accessSync(file, fs.constants.X_OK);
    return true;
  } catch (e) {
    return false;
  }
}

// "~/bin/fdev" and "${workspaceFolder}/bin/fdev" in the fdev.path setting.
function expand(p, folder) {
  let out = p;
  if (out === '~' || out.startsWith('~/') || out.startsWith('~\\')) {
    out = path.join(os.homedir(), out.slice(1));
  }
  if (folder) out = out.replace(/\$\{workspaceFolder\}/g, folder.uri.fsPath);
  return out;
}

// What `go env` says about where `go install` puts programs; "" when Go is
// not installed (or does not answer within two seconds).
function goBinDirs() {
  const dirs = [];
  try {
    const out = childProcess.execFileSync('go', ['env', 'GOBIN', 'GOPATH'], {
      encoding: 'utf8',
      timeout: 2000,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    const [gobin, gopath] = out.split(/\r?\n/);
    if (gobin) dirs.push(gobin.trim());
    if (gopath) {
      for (const p of gopath.trim().split(path.delimiter)) if (p) dirs.push(path.join(p, 'bin'));
    }
  } catch (e) {
    // no Go: the other places are enough
  }
  return dirs;
}

/**
 * findBinary returns the fdev program to run, or null: the fdev.path setting
 * (a file, or a folder that holds fdev), then PATH, $GOBIN, $GOPATH/bin,
 * ~/go/bin, the usual Homebrew and /usr/local folders (an editor started
 * from the Dock does not see the shell's PATH), and last what `go env` says.
 */
function findBinary(setting, folder) {
  const names = isWindows ? ['fdev.exe', 'fdev.cmd', 'fdev.bat'] : ['fdev'];
  if (setting) {
    const p = expand(setting, folder);
    if (isExecutable(p)) return p;
    for (const name of names) {
      const inside = path.join(p, name);
      if (isExecutable(inside)) return inside;
    }
    return null; // a path that was set on purpose is not replaced by another fdev
  }
  const lookIn = (dirs) => {
    for (const dir of dirs) {
      for (const name of names) {
        const file = path.join(dir, name);
        if (isExecutable(file)) return file;
      }
    }
    return null;
  };
  const home = os.homedir();
  const fromEnv = [];
  if (process.env.GOBIN) fromEnv.push(process.env.GOBIN);
  for (const p of (process.env.GOPATH || '').split(path.delimiter)) {
    if (p) fromEnv.push(path.join(p, 'bin'));
  }
  const usual = [path.join(home, 'go', 'bin')];
  if (!isWindows) usual.push('/opt/homebrew/bin', '/usr/local/bin', path.join(home, '.local', 'bin'));
  return (
    lookIn((process.env.PATH || '').split(path.delimiter).filter(Boolean)) ||
    lookIn(fromEnv) ||
    lookIn(usual) ||
    lookIn(goBinDirs())
  );
}

// The folder fdev starts in: the workspace folder of the active file, else
// the first workspace folder, else the folder of the active file.
function workingFolder() {
  const editor = vscode.window.activeTextEditor;
  const uri = editor && editor.document.uri.scheme === 'file' ? editor.document.uri : null;
  let folder = uri ? vscode.workspace.getWorkspaceFolder(uri) : undefined;
  if (!folder && vscode.workspace.workspaceFolders) folder = vscode.workspace.workspaceFolders[0];
  if (folder) return { folder, cwd: folder.uri.fsPath };
  return { folder: undefined, cwd: uri ? path.dirname(uri.fsPath) : undefined };
}

async function notInstalled(setting) {
  const text = setting
    ? `fdev was not found at "${setting}" (the fdev.path setting).`
    : 'fdev is not installed (or is not on the PATH, $GOBIN or ~/go/bin).';
  const pick = await vscode.window.showErrorMessage(
    `${text} Install it with: ${INSTALL}`,
    'Install in terminal',
    'Copy command',
  );
  if (pick === 'Copy command') {
    await vscode.env.clipboard.writeText(INSTALL);
  } else if (pick === 'Install in terminal') {
    const terminal = vscode.window.createTerminal({ name: 'fdev install' });
    terminal.show();
    terminal.sendText(INSTALL);
  }
}

/** open starts fdev in its terminal, or focuses the one that is running. */
async function open(settings) {
  const running = vscode.window.terminals.find(
    (t) => t.name === TERMINAL_NAME && t.exitStatus === undefined,
  );
  if (running) {
    running.show();
    return;
  }
  const { folder, cwd } = workingFolder();
  const binary = findBinary(settings.path, folder);
  if (!binary) {
    await notInstalled(settings.path);
    return;
  }
  const terminal = vscode.window.createTerminal({
    name: TERMINAL_NAME,
    shellPath: binary,
    cwd,
    env: { FDEV_FROM_VSCODE: '1' },
    iconPath: new vscode.ThemeIcon('rocket'),
    location: { viewColumn: vscode.ViewColumn.Active },
  });
  terminal.show();
}

module.exports = { open, findBinary, INSTALL };
