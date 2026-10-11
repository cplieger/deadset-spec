interface SharePayload {
  title: string;
  url: string;
}
interface Host {
  share(data: SharePayload): void;
}
interface NoticeInit {
  subject: string;
  body: string;
}
interface RelayMessage {
  topic: string;
  body: string;
}
interface Relay {
  send(m: RelayMessage): string;
}
declare function notify(init: NoticeInit): void;
declare var host: Host;
