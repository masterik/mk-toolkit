import { isTypeOf } from '../isTypeOf';

class TestClass { /**/ }
function testFunc() { /**/ }
const testClass = new TestClass();
const testProto = new testFunc();
const testDate = new Date();

describe('isTypeOf: String', () => {
  test.each([
    '', '\n', 'test'
  ])('should return true for strings: %o', value => {
    expect(isTypeOf(value, String)).toBe(true);
  });

  test.each([
    undefined,
    null,
    10,
    true,
    {},
    [''],
    testDate,
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, String)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, String)).toBe(false);
  });
});

describe('isTypeOf: Number', () => {
  test.each([
    0, 1, -1, 123.456
  ])('should return true for numbers: %o', value => {
    expect(isTypeOf(value, Number)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    true,
    {},
    [10],
    testDate,
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, Number)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, Number)).toBe(false);
  });
});

describe('isTypeOf: Boolean', () => {
  test.each([
    true, false
  ])('should return true for booleans: %o', value => {
    expect(isTypeOf(value, Boolean)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    {},
    [true],
    testDate,
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, Boolean)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, Boolean)).toBe(false);
  });
});

describe('isTypeOf: Date', () => {
  test('should return true for date', () => {
    expect(isTypeOf(testDate, Date)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    true,
    {},
    [testDate],
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, Date)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, Date)).toBe(false);
  });
});

describe('isTypeOf: Object', () => {
  test('should return true for any object', () => {
    expect(isTypeOf({}, Object)).toBe(true);
    expect(isTypeOf([], Object)).toBe(true);
    expect(isTypeOf(testClass, Object)).toBe(true);
    expect(isTypeOf(testDate, Object)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    true
  ])('should return false for primitive types: %o', value => {
    expect(isTypeOf(value, Object)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, Object)).toBe(false);
  });
});

describe('isTypeOf: Function', () => {
  test('should return true for function', () => {
    expect(isTypeOf(testFunc, Function)).toBe(true);
  });
  test('should return true for lambda', () => {
    const lambda = (p) => { /**/ };
    expect(isTypeOf(lambda, Function)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    true,
    {},
    [],
    testDate,
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, Function)).toBe(false);
  });
});

describe('isTypeOf: Class', () => {
  test('should return true when class type matches', () => {
    expect(isTypeOf(testClass, TestClass)).toBe(true);
  });
  test('should return true when prototype matches', () => {
    expect(isTypeOf(testProto, testFunc)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    true,
    {},
    [testClass],
    testDate
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, TestClass)).toBe(false);
    expect(isTypeOf(value, testFunc)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, TestClass)).toBe(false);
  });
});

describe('isTypeOf: Array', () => {
  test('should return true for array', () => {
    expect(isTypeOf([], Array)).toBe(true);
    expect(isTypeOf([1, 2], Array)).toBe(true);
    expect(isTypeOf(['', true], Array)).toBe(true);
  });

  test.each([
    undefined,
    null,
    '',
    10,
    {},
    testDate,
    testClass
  ])('should return false for others: %o', value => {
    expect(isTypeOf(value, Array)).toBe(false);
  });
  test('should return false for function', () => {
    expect(isTypeOf(testFunc, Array)).toBe(false);
  });
});
