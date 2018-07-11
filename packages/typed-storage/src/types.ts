export interface Constructable<T> {
  new(...args: any[]): T;
  new(values: Partial<T>): T;
}

export class TypedKey<T> {
  constructor(
    public readonly type: Constructable<T>,
    public readonly key: string
  ) { }
}
