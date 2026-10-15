interface Providers {
  oauth: string[];
  phone: boolean;
}

async function getProviders(): Promise<Providers> {
  return { oauth: ["github"], phone: true };
}

export async function login(): Promise<number> {
  const providers = await getProviders();
  return providers.oauth.length;
}
