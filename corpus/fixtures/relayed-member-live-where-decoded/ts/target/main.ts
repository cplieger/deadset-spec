interface Event {
  kind: string;
  seq: number;
}

const event = JSON.parse("{}") as Event;
console.log(JSON.stringify(event));
