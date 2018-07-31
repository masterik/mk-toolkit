import { TypedKey, ArrayKey } from './types';
import { resolveValue } from './resolveValue';

// tslint:disable:unified-signatures
export class TypedStorage {
  constructor(private readonly storage: Storage) { }

  getItem<T, R = T>(key: string | TypedKey<T>): R;
  getItem<T, R = T[]>(key: ArrayKey<T>): R;
  getItem<T>(key: string | TypedKey<T> | ArrayKey<T>) {
    const keyName = getStorageKeyName(key);

    const stringValue = this.storage.getItem(keyName);
    if (!stringValue) {
      return null;
    }

    try {
      const value = JSON.parse(stringValue, reviver);

      if (typeof key === 'string') {
        return value as T;
      }

      if (key instanceof ArrayKey) {
        return resolveValue(key, value as Array<Partial<T>>);
      }

      return resolveValue(key, value);
    } catch {
      return null;
    }
  }

  setItem<T>(key: string | TypedKey<T> | ArrayKey<T>, item: T | T[]): void {
    if (item === undefined || item === null) {
      return;
    }

    const storageKey = getStorageKeyName(key);
    const storageValue = JSON.stringify(item);

    this.storage.setItem(storageKey, storageValue);
  }

  removeItem<T>(key: string | TypedKey<T> | ArrayKey<T>): void {
    const storageKey = getStorageKeyName(key);
    this.storage.removeItem(storageKey);
  }

  clear(): void {
    this.storage.clear();
  }
}
// tslint:enable:unified-signatures

function getStorageKeyName<T>(key: string | TypedKey<T> | ArrayKey<T>) {
  return typeof key === 'string' ? key : key.keyName;
}

const dateFormat = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

function reviver(_, value) {
  if (typeof value === 'string' && dateFormat.test(value)) {
    return new Date(value);
  }

  return value;
}
