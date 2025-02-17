
import { WalletClient } from 'viem'
import { createSiweMessage } from 'viem/siwe'

export const authenticationDomain = 's3-aapp.kubocloud.io' // 's3.apocryph.io'
export const consoleUrl = 'https://console-s3-aapp.kubocloud.io'

export async function getSiweToken(walletClient: WalletClient, tokenExpirationSeconds?: number) {
  const wallet = walletClient.account!

  const issuedAt = new Date()
  const expiry = tokenExpirationSeconds ? issuedAt.valueOf() + tokenExpirationSeconds * 1000 : undefined

  const message = createSiweMessage({
    address: wallet.address,
    chainId: walletClient.chain!.id,
    domain: authenticationDomain,
    nonce: issuedAt.valueOf().toString(),
    uri: consoleUrl,
    version: '1',
    expirationTime: expiry ? new Date(expiry) : undefined,
  })
  const signature = await walletClient.signMessage({
    message,
    account: wallet
  })
  const token = JSON.stringify({message: message, signature: signature})
  return token
}
