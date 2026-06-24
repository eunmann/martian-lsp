import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

const RELEASE_BASE = 'https://github.com/eunmann/martian-lsp/releases/latest/download';

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const cfg = vscode.workspace.getConfiguration('martian');
  const mroPaths = cfg.get<string[]>('mroPaths', []);

  let command: string;
  try {
    command = await resolveServer(context, cfg);
  } catch (err) {
    vscode.window.showErrorMessage(
      `Martian LSP: ${err instanceof Error ? err.message : String(err)}`,
    );
    return;
  }

  const serverOptions: ServerOptions = { command, args: [] };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'mro' }],
    initializationOptions: { mroPaths },
  };

  client = new LanguageClient('martian', 'Martian LSP', serverOptions, clientOptions);
  await client.start();
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

// resolveServer picks the mrlsp binary: an explicit setting, then PATH, then a
// platform binary downloaded from GitHub Releases (cached in global storage).
async function resolveServer(
  context: vscode.ExtensionContext,
  cfg: vscode.WorkspaceConfiguration,
): Promise<string> {
  const configured = cfg.get<string>('serverPath', 'mrlsp');
  if (configured && configured !== 'mrlsp') {
    return configured;
  }
  if (onPath('mrlsp')) {
    return 'mrlsp';
  }
  return downloadServer(context);
}

function onPath(bin: string): boolean {
  const exts = process.platform === 'win32' ? ['.exe', '.cmd', ''] : [''];
  for (const dir of (process.env.PATH ?? '').split(path.delimiter)) {
    for (const ext of exts) {
      try {
        fs.accessSync(path.join(dir, bin + ext), fs.constants.X_OK);
        return true;
      } catch {
        // keep looking
      }
    }
  }
  return false;
}

function assetName(): string {
  const goarch = process.arch === 'arm64' ? 'arm64' : 'amd64';
  const goos = process.platform === 'win32' ? 'windows' : process.platform; // linux | darwin | windows
  return `mrlsp-${goos}-${goarch}${goos === 'windows' ? '.exe' : ''}`;
}

async function downloadServer(context: vscode.ExtensionContext): Promise<string> {
  const asset = assetName();
  const dir = context.globalStorageUri.fsPath;
  fs.mkdirSync(dir, { recursive: true });
  const dest = path.join(dir, asset);
  if (fs.existsSync(dest)) {
    return dest;
  }

  const url = `${RELEASE_BASE}/${asset}`;
  await vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: `Downloading mrlsp (${asset})…` },
    async () => {
      const res = await fetch(url);
      if (!res.ok) {
        throw new Error(
          `could not download ${url} (HTTP ${res.status}). ` +
            'Install it manually with: go install github.com/eunmann/martian-lsp/cmd/mrlsp@latest, ' +
            'then set "martian.serverPath".',
        );
      }
      fs.writeFileSync(dest, Buffer.from(await res.arrayBuffer()));
      if (process.platform !== 'win32') {
        fs.chmodSync(dest, 0o755);
      }
    },
  );
  return dest;
}
