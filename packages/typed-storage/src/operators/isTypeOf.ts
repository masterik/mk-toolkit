export function isTypeOf<T>(obj: any, type: { prototype: T }): obj is T;
export function isTypeOf(obj: any, type: any): boolean {
  const objType: string = typeof obj;
  const typeString = type.toString();
  const nameRegex: RegExp = /Arguments|Function|String|Number|Date|Array|Boolean|RegExp/;

  let typeName: string;

  if (obj && objType === 'object') {
    return obj instanceof type;
  }

  if (typeString.startsWith('class ')) {
    return type.name.toLowerCase() === objType;
  }

  typeName = typeString.match(nameRegex);
  if (typeName) {
    return typeName[0].toLowerCase() === objType;
  }

  return false;
}
