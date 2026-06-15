const mobilePhonePattern = /(?:\+?86[-\s]?)?(1[3-9]\d{9})/g
const landlinePhonePattern = /(^|[^0-9A-Za-z])(\d{3,4}[-\s]?\d{7,8})(?=$|[^0-9A-Za-z])/g
const addressPattern = /(省|市|区|县|镇|街道|路|号|室|村|组)/

function cleanLine(line) {
  return String(line || '')
    .replace(/[，,；;]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function valueAfterLabel(text, labels) {
  for (const label of labels) {
    const pattern = new RegExp(`(?:^|\\n)\\s*${label}[：:\\s]+([^\\n]+)`, 'i')
    const match = text.match(pattern)
    if (match?.[1]) return cleanLine(match[1])
  }
  return ''
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function likelyPersonName(token) {
  return /^[\u4e00-\u9fa5A-Za-z·]{2,16}$/.test(String(token || ''))
}

function splitTokens(text) {
  return cleanLine(text).split(' ').map((part) => cleanLine(part)).filter(Boolean)
}

function extractNameFromAddressBlock(text) {
  const tokens = splitTokens(text)
  if (tokens.length < 2) return { name: '', address: '' }
  const last = tokens[tokens.length - 1]
  const penultimate = tokens[tokens.length - 2]

  if (/^(收|收件|收件人|收货)$/.test(last) && likelyPersonName(penultimate)) {
    return { name: penultimate, address: tokens.slice(0, -2).join(' ') }
  }
  if (likelyPersonName(last) && addressPattern.test(tokens.slice(0, -1).join(' '))) {
    return { name: last, address: tokens.slice(0, -1).join(' ') }
  }
  return { name: '', address: '' }
}

function findPhone(text) {
  mobilePhonePattern.lastIndex = 0
  const mobile = mobilePhonePattern.exec(text)
  if (mobile) {
    return { text: mobile[0], value: mobile[1], index: mobile.index }
  }
  landlinePhonePattern.lastIndex = 0
  const landline = landlinePhonePattern.exec(text)
  if (!landline) return null
  const prefix = landline[1] || ''
  return {
    text: landline[2],
    value: landline[2],
    index: landline.index + prefix.length,
  }
}

export function parseRecipientText(input) {
  const raw = String(input || '').trim()
  if (!raw) return { recipient_name: '', phone: '', address: '' }

  const normalized = raw.replace(/\r/g, '\n')
  const phoneMatch = findPhone(normalized)
  const phone = phoneMatch ? cleanLine(phoneMatch.value || '') : ''
  const labeledName = valueAfterLabel(normalized, ['收件人', '姓名', '联系人', '客户'])
  const labeledAddress = valueAfterLabel(normalized, ['收货地址', '详细地址', '地址'])

  let withoutPhone = normalized
  if (phoneMatch?.text) withoutPhone = `${withoutPhone.slice(0, phoneMatch.index)} ${withoutPhone.slice(phoneMatch.index + phoneMatch.text.length)}`
  const compact = withoutPhone
    .split('\n')
    .map((line) => cleanLine(line.replace(/^(收件人|姓名|联系人|客户|电话|手机|联系方式|收货地址|详细地址|地址)[：:\s]*/i, '')))
    .filter(Boolean)

  let recipientName = labeledName
  const beforePhone = phoneMatch ? cleanLine(normalized.slice(0, phoneMatch.index)) : ''
  const afterPhone = phoneMatch ? cleanLine(normalized.slice((phoneMatch.index || 0) + phoneMatch.text.length)) : ''
  const fromAddressBlock = extractNameFromAddressBlock(beforePhone)
  if (!recipientName && phoneMatch) {
    if (addressPattern.test(beforePhone) && fromAddressBlock.name) {
      recipientName = fromAddressBlock.name
    } else if (addressPattern.test(beforePhone) && afterPhone && !addressPattern.test(afterPhone)) {
      recipientName = afterPhone.split(' ')[0] || ''
    } else {
      recipientName = beforePhone.replace(/^(收件人|姓名|联系人|客户)[：:\s]*/i, '').split(' ')[0] || ''
    }
  }
  if (!recipientName) {
    const first = compact.find((line) => !addressPattern.test(line) && !/地址/.test(line))
    recipientName = cleanLine(first || '').split(' ')[0] || ''
  }

  let address = labeledAddress
  if (!address && phoneMatch && addressPattern.test(beforePhone) && fromAddressBlock.address) {
    address = fromAddressBlock.address
  }
  if (!address && phoneMatch && addressPattern.test(beforePhone) && afterPhone && recipientName === afterPhone.split(' ')[0]) {
    address = beforePhone
  }
  if (!address) {
    const addressLine = compact.find((line) => addressPattern.test(line))
    address = addressLine || compact.filter((line) => line !== recipientName).join(' ')
  }
  if (recipientName) {
    const trailingName = new RegExp(`\\s*${escapeRegExp(recipientName)}\\s*(?:收件人|收件|收货|收)?\\s*$`, 'i')
    address = cleanLine(address.replace(trailingName, ' '))
  }
  if (recipientName && address.startsWith(recipientName)) {
    address = cleanLine(address.slice(recipientName.length))
  }

  return {
    recipient_name: recipientName,
    phone,
    address: cleanLine(address),
  }
}
