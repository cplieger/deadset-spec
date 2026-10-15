interface Event {
  kind: string;
  seq: number;
}

interface Status {
  spare: string;
}

const status = {} as Status;
const event = JSON.parse("{}") as Event;
console.log(JSON.stringify(status), JSON.stringify(event));
