const phonePattern = /(?:\+?86[-\s]?)?(1[3-9]\d{9})|(\d{3,4}[-\s]?\d{7,8})/

function cleanLine(line) {
  return String(line || '')
    .replace(/[，,；;]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function valueAfterLabel(text, labels) {
  for (const label of labels) {
    const pattern = new RegExp(`${label}[：:\\s]*([^\\n]+)`, 'i')
    const match = text.match(pattern)
    if (match?.[1]) return cleanLine(match[1])
  }
  return ''
}

export function parseRecipientText(input) {
  const raw = String(input || '').trim()
  if (!raw) return { recipient_name: '', phone: '', address: '' }

  const normalized = raw.replace(/\r/g, '\n')
  const phoneMatch = normalized.match(phonePattern)
  const phone = phoneMatch ? cleanLine(phoneMatch[1] || phoneMatch[2] || '') : ''
  const labeledName = valueAfterLabel(normalized, ['收件人', '姓名', '联系人', '客户'])
  const labeledAddress = valueAfterLabel(normalized, ['收货地址', '详细地址', '地址'])

  let withoutPhone = normalized
  if (phoneMatch?.[0]) withoutPhone = withoutPhone.replace(phoneMatch[0], ' ')
  const compact = withoutPhone
    .split('\n')
    .map((line) => cleanLine(line.replace(/^(收件人|姓名|联系人|客户|电话|手机|联系方式|收货地址|详细地址|地址)[：:\s]*/i, '')))
    .filter(Boolean)

  let recipientName = labeledName
  if (!recipientName && phoneMatch) {
    recipientName = cleanLine(normalized.slice(0, phoneMatch.index)).replace(/^(收件人|姓名|联系人|客户)[：:\s]*/i, '').split(' ')[0] || ''
  }
  if (!recipientName) {
    const first = compact.find((line) => !/(省|市|区|县|镇|街道|路|号|室|地址)/.test(line))
    recipientName = cleanLine(first || '').split(' ')[0] || ''
  }

  let address = labeledAddress
  if (!address) {
    const addressLine = compact.find((line) => /(省|市|区|县|镇|街道|路|号|室|村|组)/.test(line))
    address = addressLine || compact.filter((line) => line !== recipientName).join(' ')
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
