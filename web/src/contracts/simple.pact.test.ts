/**
 * Simple Pact test to verify setup
 */

describe('Pact Setup Test', () => {
  it('should be able to run a simple test', () => {
    expect(true).toBe(true);
  });
  
  it('should have fetch available', () => {
    expect(global.fetch).toBeDefined();
  });
});