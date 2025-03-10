// SPDX-License-Identifier: GPL-3.0

import { defineConfig } from '@wagmi/cli'
import { foundry } from '@wagmi/cli/plugins'

export default defineConfig({
  out: './generated.ts',
  contracts: [],
  plugins: [
    foundry({
      project: '../../contracts',
      forge: {
        build: false
      },
      include: [
        'MockToken.json',
        'ERC20.json',
        'PaymentV2.json',
      ]
    })
  ]
})
