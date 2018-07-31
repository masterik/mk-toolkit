import { Constructor } from '../types';

export class TypedKey<T> {
  constructor(
    readonly ctor: Constructor<T>,
    readonly keyName: string
  ) { }
}

export class ArrayKey<T> {
  constructor(
    readonly ctor: Constructor<T>,
    readonly keyName: string
  ) { }
}
