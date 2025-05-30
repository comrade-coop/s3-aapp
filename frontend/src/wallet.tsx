import { Chain, defineChain } from 'viem'
import { createConfig, http } from 'wagmi'
import { base } from 'wagmi/chains'
import { metaMask } from 'wagmi/connectors'

const chain = import.meta.env.VITE_CHAIN_CONFIG ? defineChain(JSON.parse(import.meta.env.VITE_CHAIN_CONFIG) as Chain) : base
 
export const config = createConfig({
  chains: [chain],
  connectors: [metaMask()],
  transports: {
    [chain.id]: http(),
  },
})
