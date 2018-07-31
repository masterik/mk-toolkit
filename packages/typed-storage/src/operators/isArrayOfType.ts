import { isTypeOf } from './isTypeOf';

export function isArrayOfType<T>(arr: T[], type: { prototype: T }): arr is T[];
export function isArrayOfType(arr: any, type: any): boolean {
  return isTypeOf(arr, Array) && (arr as any[]).every(x => isTypeOf(x, type));
}
