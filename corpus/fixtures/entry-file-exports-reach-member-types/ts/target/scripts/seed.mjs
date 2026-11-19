// A file a package script only runs: node reads none of its exports.
class Seeder {
  plant() {
    return 0;
  }
}

export const seeder = { make: Seeder };
