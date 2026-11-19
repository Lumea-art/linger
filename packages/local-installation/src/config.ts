export function windowsSystemRoot(): string {
  return process.env.SystemRoot ?? "C:\\Windows";
}
