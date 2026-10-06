// The operations of a service, each member the result one operation returns.
export interface Operations {
  listUsers: string[];
  getUser: string;
  renameUser: boolean;
  suspendUser: boolean;
  removeUser: number;
  deleteUser: null;
}
