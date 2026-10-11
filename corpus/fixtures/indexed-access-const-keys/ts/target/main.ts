import { createBus } from "@example/bus";

interface Events {
  openSeries: number;
  openMovie: number;
  closed: string;
  dropped: boolean;
}

const BusEvent = { OpenSeries: "openSeries", OpenMovie: "openMovie" } as const;

export const { emit, on, once } = createBus<Events>();

emit(BusEvent.OpenSeries, 1);
on(BusEvent.OpenMovie, (n) => console.log(n));
once("closed");
