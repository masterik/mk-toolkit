import { TypedKey, ArrayKey } from './types';
import { createInstance } from '../operators/createInstance';

export function resolveValue<T>(storageKey: TypedKey<T>, rawValue: Partial<T>): T;
export function resolveValue<T>(storageKey: ArrayKey<T>, rawValue: Array<Partial<T>>): T[];
export function resolveValue<T>(storageKey: TypedKey<T> | ArrayKey<T>, rawValue: any) {
  if (!rawValue) {
    return null;
  }

  if (storageKey instanceof ArrayKey) {
    const array = rawValue as Array<Partial<T>>;
    return array.map(item => createInstance(storageKey.ctor, item));
  }

  return createInstance(storageKey.ctor, rawValue);
}
