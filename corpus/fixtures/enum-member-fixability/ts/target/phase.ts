export enum Phase {
  Idle,
  Running,
  Stopped,
  Failed = 9,
  Paused,
  Closed,
}

export enum Spare {
  On,
  Off,
}

export enum Tier {
  Base = 5,
  Next,
  Alias = Next,
  Top,
}
