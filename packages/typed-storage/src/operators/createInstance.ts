import { Constructor } from '../types';

export function createInstance(ctor: BooleanConstructor, value: boolean): boolean;
export function createInstance(ctor: NumberConstructor, value: number): number;
export function createInstance(ctor: StringConstructor, value: string): string;
export function createInstance(ctor: DateConstructor, value: Date): Date;
export function createInstance<T>(ctor: Constructor<T>, value: Partial<T>): T;
export function createInstance(ctor: any, value: any) {
  if (!ctor) {
    return undefined;
  }

  if (ctor === Date) {
    return dateOrUndefined(value);
  }

  if (ctor === Number) {
    return typeof value === 'number' ? value : undefined;
  }

  if (ctor === Boolean) {
    return typeof value === 'boolean' ? value : undefined;
  }

  if (ctor === String) {
    return typeof value === 'string' ? value : undefined;
  }

  const instance = new ctor();

  if (value) {
    Object.assign(instance, value);
  }

  return instance;
}



const dateFormat = /^\"?(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z)\"?$/;

function dateOrUndefined(value: any) {
  if (value instanceof Date) {
    return value;
  } else if (typeof value === 'string') {
    const match = dateFormat.exec(value);

    if (match) {
      return new Date(match[1]);
    }
  }

  return undefined;
}
