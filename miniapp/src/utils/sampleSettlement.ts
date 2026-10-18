export function isFreeSampleOrder(sample: boolean, prices: number[], shipping: number, other: number, total: number): boolean {
 return sample && prices.length>0 && prices.every(value=>Number.isFinite(value) && value===0) && shipping===0 && other===0 && total===0
}
