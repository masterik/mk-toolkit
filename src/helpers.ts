export function isPrimitive(value: any): boolean {
  return typeof value === 'string'
    || typeof value === 'number'
    || typeof value === 'boolean'
    || value instanceof Date;
}
