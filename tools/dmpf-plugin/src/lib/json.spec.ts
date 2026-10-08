import { parseJsonOf } from './json';

describe('[lib] json', () => {
  it('should accept the comments and trailing commas that nx.json allows', () => {
    expect(parseJsonOf('{\n  // comment\n  "a": 1,\n}', 'nx.json')).toEqual({ a: 1 });
  });

  it('should name the file when the content is not JSON, as a merge conflict leaves it', () => {
    expect(() => parseJsonOf('<<<<<<< HEAD\n{}', 'dmpf.rendered.json')).toThrow(
      'dmpf.rendered.json',
    );
  });
});
