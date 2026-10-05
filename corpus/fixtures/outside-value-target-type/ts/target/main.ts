// Alert is the target's view of a notification the platform creates.
interface Alert {
  onclick: ((event: Event) => unknown) | null;
  label?: string;
}

const notice: Alert = new Notification("ready");
notice.onclick = () => true;
notice.label = "ready";

// The entry exports the alert it created.
export const shown = notice;
