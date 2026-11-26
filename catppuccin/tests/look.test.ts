import { expect, test } from 'claude-code/testing'

import { buildLook } from '../hooks/register'
import { ACCENTS, DARK_FLAVORS } from '../hooks/palette'

test('every flavor and accent paints on Desktop', () => {
  for (const flavor of DARK_FLAVORS) {
    for (const accent of ACCENTS) {
      const { problems } = buildLook(flavor, accent)
      expect(problems.filter(line => line.startsWith("Desktop won't paint"))).toEqual([])
    }
  }
})
