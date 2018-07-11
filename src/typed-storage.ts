import { TypedKey } from './types';

export class TypedStorage {
  constructor(private readonly storage: Storage) { }

  getItem<T = string>(key: string | TypedKey<T>): T {
    const storageKey = getStorageKey(key);

    const stringValue = this.storage.getItem(storageKey);
    if (!stringValue) {
      return null;
    }

    try {
      const value: T = JSON.parse(stringValue);

      if (typeof key === 'string') {
        return value;
      } else {
        return new key.type(value);
      }
    } catch {
      return null;
    }
  }

  setItem<T>(key: string | TypedKey<T>, item: T): void {
    if (item === undefined || item === null || typeof item === 'function') {
      return;
    }

    const storageKey = getStorageKey(key);
    const storageValue = JSON.stringify(item);

    this.storage.setItem(storageKey, storageValue);
  }

  removeItem<T>(key: string | TypedKey<T>): void {
    const storageKey = getStorageKey(key);
    this.storage.removeItem(storageKey);
  }

  clear(): void {
    this.storage.clear();
  }
}

function getStorageKey<T>(key: string | TypedKey<T>) {
  return typeof key === 'string' ? key : key.key;
}
