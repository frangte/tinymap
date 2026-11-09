// Text of rides.ts: a Crab Bike trip as the game tells it (chat lines, phone notices, the action button, the tracker).
import { catalog } from './index';
import type { RideError } from '../../net/rideProtocol';

/** how a trip ended (the server's `end`) */
type End =
  | 'alighted'
  | 'cancelled_by_passenger'
  | 'cancelled_by_driver'
  | 'no_driver'
  | 'pickup_timeout'
  | 'passenger_left'
  | 'driver_left'
  | 'driver_got_off'
  | 'passenger_got_off'
  | 'hold_failed'
  | 'trip_timeout';

/** the passenger's line and the driver's line ('' for no line) */
type Lines = readonly [string, string];

/** The line for one side of a trip that ended this way; `fallback` for an end this build does not know. */
function endLine(ends: Record<End, Lines>, fallback: Lines, why: string, passenger: boolean): string {
  const pair = (ends as Record<string, Lines | undefined>)[why] ?? fallback;
  return pair[passenger ? 0 : 1];
}

const VI_ERRORS: Record<RideError, string> = {
  no_account: 'Đặt xe cần tài khoản: tải lại trang để thử lại.',
  busy: 'Bạn đang có một chuyến rồi.',
  unknown_place: 'Không tìm thấy nơi này.',
  no_bike: 'Tài xế phải đang ngồi trên xe của mình.',
  too_far: 'Lại gần xe hơn chút nữa nhé.',
  not_stopped: 'Đợi xe dừng hẳn đã.',
  not_there: 'Chưa tới nơi: chở khách vào gần hơn.',
  balance: 'Ví không đủ tiền.',
  gone: 'Cuốc này không còn nữa.',
  not_allowed: 'Chỉ tài xế Crab mới nhận cuốc được.',
  tip: 'Tiền tip không hợp lệ.',
  too_soon: 'Vừa đặt xong, đợi vài giây rồi đặt lại nhé.',
  riding: 'Xuống xe của bạn trước đã.',
  here: 'Bạn đang ở đó rồi.',
  unknown: 'Có lỗi, thử lại nhé.',
};

const EN_ERRORS: Record<RideError, string> = {
  no_account: 'Booking a ride needs an account: reload the page to try again.',
  busy: 'You already have a ride.',
  unknown_place: "Can't find that place.",
  no_bike: 'The driver must be sitting on their own bike.',
  too_far: 'Get a little closer to the bike.',
  not_stopped: 'Wait for the bike to stop.',
  not_there: 'Not there yet: bring the passenger closer.',
  balance: 'Not enough money in your wallet.',
  gone: 'This ride is no longer available.',
  not_allowed: 'Only Crab drivers can take rides.',
  tip: "That tip isn't valid.",
  too_soon: 'You just booked: wait a few seconds before booking again.',
  riding: 'Get off your own bike first.',
  here: "You're already there.",
  unknown: 'Something went wrong, please try again.',
};

const VI_ENDS: Record<End, Lines> = {
  alighted: ['Bạn đã xuống xe giữa đường.', 'Khách đã xuống xe giữa đường.'],
  cancelled_by_passenger: ['Đã huỷ chuyến, tiền đã hoàn lại.', 'Khách đã huỷ chuyến.'],
  cancelled_by_driver: ['Tài xế đã huỷ cuốc.', 'Bạn đã huỷ cuốc.'],
  no_driver: ['Chưa có tài xế nào nhận. Tiền đã hoàn lại, thử lại sau nhé.', ''],
  pickup_timeout: ['Tài xế không tới kịp. Tiền đã hoàn lại.', 'Quá giờ chưa đón khách: cuốc đã huỷ.'],
  passenger_left: ['', 'Khách đã rời phố.'],
  driver_left: ['Tài xế đã rời phố. Tiền đã hoàn lại.', ''],
  driver_got_off: ['Tài xế xuống xe giữa đường. Tiền đã hoàn lại.', 'Bạn xuống xe giữa đường: khách được hoàn tiền.'],
  passenger_got_off: ['Bạn đã rời xe giữa đường.', 'Khách đã rời xe giữa đường.'],
  hold_failed: ['Chưa đặt được xe, thử lại nhé.', ''],
  trip_timeout: ['Chuyến đi quá lâu nên đã huỷ, tiền đã hoàn lại.', 'Chở khách quá lâu: cuốc đã huỷ, khách được hoàn tiền.'],
};
const VI_END_FALLBACK: Lines = ['Chuyến đi đã kết thúc.', 'Cuốc xe đã kết thúc.'];

const EN_ENDS: Record<End, Lines> = {
  alighted: ['You got off on the way.', 'The passenger got off on the way.'],
  cancelled_by_passenger: ['Trip cancelled, your money has been refunded.', 'The passenger cancelled the trip.'],
  cancelled_by_driver: ['The driver cancelled the ride.', 'You cancelled the ride.'],
  no_driver: ['No driver took it. Your money has been refunded; try again later.', ''],
  pickup_timeout: ["The driver didn't make it in time. Your money has been refunded.", 'Too late to pick up the passenger: the ride was cancelled.'],
  passenger_left: ['', 'The passenger has left.'],
  driver_left: ['The driver has left. Your money has been refunded.', ''],
  driver_got_off: ['The driver got off on the way. Your money has been refunded.', 'You got off on the way: the passenger gets a refund.'],
  passenger_got_off: ['You left the bike on the way.', 'The passenger left the bike on the way.'],
  hold_failed: ["Couldn't book the ride, please try again.", ''],
  trip_timeout: ['The trip took too long and was cancelled; your money has been refunded.', 'The ride took too long: it was cancelled and the passenger refunded.'],
};
const EN_END_FALLBACK: Lines = ['The trip is over.', 'The ride is over.'];

const vi = {
  error: (code: RideError): string => VI_ERRORS[code],
  /** how the trip ended, for the passenger or the driver ('' for nothing to say) */
  end: (why: string, passenger: boolean): string => endLine(VI_ENDS, VI_END_FALLBACK, why, passenger),
  rated: 'Cảm ơn bạn đã đánh giá tài xế!',
  offlineBook: 'Đang mất kết nối, chưa đặt xe được.',
  offline: 'Đang mất kết nối.',
  // the action button
  stepAlight: 'Xuống xe',
  stepBoard: 'Lên xe Crab',
  stepDropOff: 'Trả khách',
  confirmGetOff: 'Xuống xe giữa đường là huỷ cuốc, khách được hoàn tiền. Bấm lần nữa để xuống xe.',
  stopFirst: 'Dừng xe hẳn rồi trả khách.',
  // the tracker
  goalPickUp: (passenger: string) => `Đón ${passenger}`,
  goalCarryTo: (place: string) => `Chở khách tới ${place}`,
  goalFinding: 'Đang tìm tài xế…',
  goalDriverComing: (driver: string | null) => `${driver ?? 'Tài xế'} đang tới đón bạn`,
  goalGoingTo: (place: string) => `Đang tới ${place}`,
  // how the trip goes
  arrivedTitle: 'Tới nơi',
  arrivedPaid: (place: string, fare: string) => `${place}. Đã trả ${fare}.`,
  alightedPaid: (fare: string) => `Xuống xe giữa đường, đã trả ${fare}.`,
  tripDoneTitle: 'Hoàn thành cuốc xe',
  tripDone: (passenger: string, place: string, paid: string) => `Chở ${passenger} tới ${place}. Nhận ${paid}.`,
  earned: (line: string, paid: string) => `${line} Nhận ${paid}.`,
  acceptedRider: (passenger: string) => `Đã nhận cuốc: tới đón ${passenger}.`,
  boardedRider: (passenger: string, place: string) => `${passenger} đã lên xe. Chở tới ${place}, tới nơi thì dừng xe và bấm Trả khách.`,
  driverCancelled: 'Tài xế đã huỷ, đang tìm tài xế khác…',
  requested: (place: string) => `Đã đặt xe tới ${place}, đang tìm tài xế…`,
  driverFoundTitle: 'Đã có tài xế',
  driverFound: (driver: string) => `${driver} đang tới đón bạn. Xe tới thì bấm Lên xe Crab.`,
  onTrip: (place: string) => `Lên xe rồi! Đang tới ${place}.`,
};

const en: typeof vi = {
  error: (code) => EN_ERRORS[code],
  end: (why, passenger) => endLine(EN_ENDS, EN_END_FALLBACK, why, passenger),
  rated: 'Thanks for rating your driver!',
  offlineBook: "Connection lost: you can't book a ride right now.",
  offline: 'Connection lost.',
  stepAlight: 'Get off',
  stepBoard: 'Get on the Crab',
  stepDropOff: 'Drop off',
  confirmGetOff: 'Getting off before you arrive cancels the ride and refunds the passenger. Press again to get off.',
  stopFirst: 'Come to a full stop to drop off the passenger.',
  goalPickUp: (passenger) => `Pick up ${passenger}`,
  goalCarryTo: (place) => `Take the passenger to ${place}`,
  goalFinding: 'Looking for a driver…',
  goalDriverComing: (driver) => (driver ? `${driver} is coming to pick you up` : 'Your driver is coming to pick you up'),
  goalGoingTo: (place) => `On the way to ${place}`,
  arrivedTitle: 'Arrived',
  arrivedPaid: (place, fare) => `${place}. Paid ${fare}.`,
  alightedPaid: (fare) => `Got off on the way, paid ${fare}.`,
  tripDoneTitle: 'Ride complete',
  tripDone: (passenger, place, paid) => `Took ${passenger} to ${place}. Earned ${paid}.`,
  earned: (line, paid) => `${line} Earned ${paid}.`,
  acceptedRider: (passenger) => `Ride taken: go pick up ${passenger}.`,
  boardedRider: (passenger, place) => `${passenger} is on. Ride to ${place}, then stop and press Drop off.`,
  driverCancelled: 'The driver cancelled. Looking for another driver…',
  requested: (place) => `Ride to ${place} booked. Looking for a driver…`,
  driverFoundTitle: 'Driver found',
  driverFound: (driver) => `${driver} is coming to pick you up. When the bike arrives, press Get on the Crab.`,
  onTrip: (place) => `You're on! Heading to ${place}.`,
};

export const L = catalog({ vi, en });
