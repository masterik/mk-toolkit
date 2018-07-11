export type Constructor<T = {}> = new (...args: any[]) => T;
export type CopyConstructor<T = {}> = new (values?: Partial<T>) => T;
export type Serializable = string | number | boolean | Date;

export class TypedKey<T> {
  constructor(
    public readonly type: CopyConstructor<T>,
    public readonly key: string
  ) { }
}
