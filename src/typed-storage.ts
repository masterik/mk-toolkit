import { TypedKey } from './types';
import { isPrimitive } from './helpers';

export class TypedStorage {
  constructor(private readonly storage: Storage) { }

  // getItem<T extends Primitive>(key: string): T
  // getItem<T extends Copyable>(key: TypedKey<T>): T
  // getItem<T>(key: string | TypedKey<T>): T {
  //   const stringValue = this.storage.getItem(key);
  //   if (!stringValue) {
  //     return null;
  //   }

  //   try {
  //     const value: T = JSON.parse(stringValue);

  //     if (klass) {
  //       return new klass(value);
  //     } else {
  //       return value;
  //     }
  //   } catch (error) {
  //     return null;
  //   }
  // }

  setItem<T>(key: string | TypedKey<T>, item: any): void {
    if (item === undefined || item === null || typeof item === 'function') {
      return;
    }

    const storageKey = typeof key === 'string' ? key : key.key;
    const storageValue = isPrimitive(item)
      ? '' + item
      : JSON.stringify(item);

    this.storage.setItem(storageKey, storageValue);
  }
}
