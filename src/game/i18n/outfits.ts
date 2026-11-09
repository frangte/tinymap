// Names of the bodies, outfits and hats (outfits.ts): the dressing panel, the outfit toast, the profile app.
import { catalog } from './index';

const bodyVi = { man: 'Nam', woman: 'Nữ' };
const bodyEn: typeof bodyVi = { man: 'Man', woman: 'Woman' };
export const BODY_NAMES = catalog({ vi: bodyVi, en: bodyEn });

const hatVi = {
  nonla: 'Nón lá',
  cap: 'Nón kết',
  bucket: 'Mũ tai bèo',
  helmet: 'Mũ bảo hiểm',
  bun: 'Tóc búi',
  none: 'Không nón',
};
const hatEn: typeof hatVi = {
  nonla: 'Conical hat',
  cap: 'Cap',
  bucket: 'Bucket hat',
  helmet: 'Helmet',
  bun: 'Hair bun',
  none: 'No hat',
};
export const HAT_NAMES = catalog({ vi: hatVi, en: hatEn });

const outfitVi = {
  thun: 'Áo thun',
  aodai: 'Áo dài',
  baba: 'Áo bà ba',
  somi: 'Sơ mi',
  xeom: 'Áo khoác xe ôm',
  hoa: 'Áo hoa',
  /** the men's beachwear; women wear the bikini */
  bikini: 'Quần bơi',
  bikiniWoman: 'Bikini',
  crab: 'Áo Crab Bike',
};
const outfitEn: typeof outfitVi = {
  thun: 'T-shirt',
  aodai: 'Áo dài',
  baba: 'Áo bà ba',
  somi: 'Shirt',
  xeom: 'Xe ôm jacket',
  hoa: 'Floral shirt',
  bikini: 'Swim trunks',
  bikiniWoman: 'Bikini',
  crab: 'Crab Bike jacket',
};
export const OUTFIT_NAMES = catalog({ vi: outfitVi, en: outfitEn });
