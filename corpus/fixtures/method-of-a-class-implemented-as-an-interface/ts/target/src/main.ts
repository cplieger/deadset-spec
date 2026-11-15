class Repo {
  get(): number {
    return 1;
  }
}
class Implemented implements Repo {
  get(): number {
    return 2;
  }
  spare(): number {
    return 0;
  }
}
const repo: Repo = new Implemented();
export const out = repo.get();
