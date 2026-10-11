declare const client: Client;

interface Pushed {
  kind: string;
}

interface Small {
  tiny: string;
}

interface Relayed {
  hop: number;
}

interface Hidden {
  unread: string;
}

class Discard implements Sink {
  put(message: unknown): void {
    void message;
  }
}

client.postMessage({ kind: "push" } satisfies Pushed);
const small: Small = { tiny: "s" };
client.postMessage(small as unknown);
const relayed: Relayed = { hop: 1 };
relay(relayed);
const sink: Sink = new Discard();
const hidden: Hidden = { unread: "h" };
sink.put(hidden);
