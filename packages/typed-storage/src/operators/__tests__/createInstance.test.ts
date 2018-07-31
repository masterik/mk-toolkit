import { createInstance } from '../createInstance';
import { isTypeOf } from '../isTypeOf';

describe('createInstance', () => {
  test('should return instance of class with default constructor', () => {
    const instance = createInstance(WithoutConstructor, {});

    expect(instance).toBeTruthy();
    expect(instance instanceof WithoutConstructor).toBe(true);
  });

  test('should return instance of class with parameterless constructor', () => {
    const instance = createInstance(ParameterlessConstructor, {});

    expect(instance).toBeTruthy();
    expect(instance instanceof ParameterlessConstructor).toBe(true);
  });

  test('should return instance of class with constructor with parameters', () => {
    const instance = createInstance(ConstructorWithParameters, {});

    expect(instance).toBeTruthy();
    expect(instance instanceof ConstructorWithParameters).toBe(true);
  });

  test('should return value for valid date', () => {
    const stringDate = JSON.stringify(new Date(2018, 7, 11, 0, 0, 0));
    const date1 = createInstance(Date, stringDate as any);
    expect(isTypeOf(date1, Date)).toBe(true);
    expect(date1).toEqual(new Date(2018, 7, 11, 0, 0, 0));
  });

  test('should return value for primitive types with truthy value', () => {
    const numberValue = createInstance(Number, 10);
    expect(typeof numberValue === 'number').toBe(true);
    expect(numberValue).toBe(10);

    const stringValue = createInstance(String, 'test');
    expect(typeof stringValue === 'string').toBe(true);
    expect(stringValue).toBe('test');

    const boolValue = createInstance(Boolean, true);
    expect(typeof boolValue === 'boolean').toBe(true);
    expect(boolValue).toBe(true);
  });

  test('should return value for primitive types with falsy value', () => {
    const numberValue = createInstance(Number, 0);
    expect(typeof numberValue === 'number').toBe(true);
    expect(numberValue).toBe(0);

    const stringValue = createInstance(String, '');
    expect(typeof stringValue === 'string').toBe(true);
    expect(stringValue).toBe('');

    const boolValue = createInstance(Boolean, false);
    expect(typeof boolValue === 'boolean').toBe(true);
    expect(boolValue).toBe(false);
  });

  test('should return undefined for primitive types with undefined value', () => {
    const numberValue = createInstance(Number, undefined);
    expect(typeof numberValue === 'undefined').toBe(true);
    expect(numberValue).toBe(undefined);

    const stringValue = createInstance(String, undefined);
    expect(typeof stringValue === 'undefined').toBe(true);
    expect(stringValue).toBe(undefined);

    const boolValue = createInstance(Boolean, undefined);
    expect(typeof boolValue === 'undefined').toBe(true);
    expect(boolValue).toBe(undefined);

    const dateValue = createInstance(Date, undefined);
    expect(typeof dateValue === 'undefined').toBe(true);
    expect(dateValue).toBe(undefined);
  });

  test('should return class with initial member values when undefined value', () => {
    const instance = createInstance(WithoutConstructor, undefined);

    expect(instance).toBeTruthy();
    expect(instance instanceof WithoutConstructor).toBe(true);
    expect(instance.foo).toBe('test');
    expect(instance.bar).toBe(undefined);
  });

  test('should return populated class when value is of compatible type', () => {
    const instance = createInstance(WithoutConstructor, { foo: 'test', bar: 10 });

    expect(instance).toBeTruthy();
    expect(instance.foo).toBe('test');
    expect(instance.bar).toBe(10);
  });

  test('should return undefined when undefined constructor', () => {
    const instance = createInstance(undefined, {});

    expect(instance).toBeFalsy();
    expect(typeof instance === 'undefined').toBe(true);
  });
});



class WithoutConstructor {
  readonly foo: string = 'test';
  readonly bar: number;
  // readonly ddd: Date = new Date(2018, 7, 11, 0, 0, 0);
}
class ParameterlessConstructor {
  readonly foo: string;
  readonly bar: number;
  // readonly ddd: Date;

  constructor() {
    this.foo = '';
    this.bar = 0;
    // this.ddd = new Date(2018, 7, 11, 0, 0, 0);
  }
}
class ConstructorWithParameters {
  readonly foo: string;
  readonly bar: number;
  // readonly ddd: Date;

  constructor(foo: string, bar: number, ddd: Date) {
    this.foo = foo;
    this.bar = bar;
    // this.ddd = ddd;
  }
}
