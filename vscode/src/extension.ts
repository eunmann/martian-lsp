import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export function activate(_context: vscode.ExtensionContext): void {
  const cfg = vscode.workspace.getConfiguration('martian');
  const command = cfg.get<string>('serverPath', 'mrlsp');
  const mroPaths = cfg.get<string[]>('mroPaths', []);

  const serverOptions: ServerOptions = { command, args: [] };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'mro' }],
    initializationOptions: { mroPaths },
  };

  client = new LanguageClient('martian', 'Martian LSP', serverOptions, clientOptions);
  void client.start();
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}
