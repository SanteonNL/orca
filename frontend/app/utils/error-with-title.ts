export class ErrorWithTitle extends Error {
  title?: string;
  userMessage?: string;

  constructor(message: string, title: string, userMessage?: string) {
    super(message);
    this.title = title;
    this.userMessage = userMessage;
  }
}
