interface Job {
  id: string;
  state: string;
}

async function request<T>(url: string): Promise<T> {
  const res = await fetch(url);
  return (await res.json()) as T;
}

export async function first(): Promise<string> {
  const job = await request<Job>("https://example.invalid/job");
  return job.id;
}
