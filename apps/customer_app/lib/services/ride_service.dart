import 'dart:math' as math;

import 'package:latlong2/latlong.dart';
import 'package:uuid/uuid.dart';

import '../models/ride_order.dart';
import 'api_client.dart';

/// Flag fitur sementara (Phase 2 mobile).
///
/// TODO(v2): Set `false` setelah backend mengimplementasikan:
///   1. GET /rides/{order_id} (API_CONTRACT 7.4) — BELUM ada di
///      internal/ride/handler.go (baru BookRide / AcceptOrder / UpdateStatus).
///   2. Endpoint info driver (nama, rating, kendaraan) & lokasi driver real
///      untuk marker bergerak di peta.
/// Selama endpoint di atas belum tersedia, tracking memakai simulasi
/// `RideMockSimulator` sehingga demo customer app tetap berjalan.
const bool kUseMockRideData = false;

class RideService {
  const RideService(this.apiClient);

  final ApiClient apiClient;

  /// POST /rides/book (API_CONTRACT 7.1). Menerima koordinat + metode
  /// pembayaran; dikirim dengan X-Idempotency-Key untuk menjamin idempotensi.
  Future<BookRideResult> bookRide({
    required double pickupLat,
    required double pickupLng,
    required double dropoffLat,
    required double dropoffLng,
    required String paymentMethod,
    required String idempotencyKey,
  }) async {
    final res = await apiClient.post(
      '/api/v1/rides/book',
      headers: {'X-Idempotency-Key': idempotencyKey},
      data: {
        'pickup_lat': pickupLat,
        'pickup_lng': pickupLng,
        'dropoff_lat': dropoffLat,
        'dropoff_lng': dropoffLng,
        'payment_method': paymentMethod,
      },
    );
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    return BookRideResult.fromJson(
      data is Map<String, dynamic> ? data : const {},
    );
  }

  /// GET /rides/{order_id} (API_CONTRACT 7.4) untuk polling status.
  Future<RideOrder> getRideDetails(String orderId) async {
    final res = await apiClient.get('/api/v1/rides/$orderId');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk ride $orderId');
    }
    return RideOrder.fromJson(data);
  }

  /// PATCH /rides/{order_id}/status untuk membatalkan ride (reason
  /// default CUSTOMER_CANCEL sesuai state machine ROADMAP 02 bagian 2.4).
  Future<void> cancelRide(String orderId, {String reason = 'CUSTOMER_CANCEL'}) async {
    await apiClient.patch(
      '/api/v1/rides/$orderId/status',
      data: {'status': 'CANCELLED', 'reason': reason},
    );
  }
}

/// Simulator untuk demo customer app selama endpoint tracking & lokasi
/// driver belum tersedia di backend. Seluruh data diberi penanda isMock,
/// dan semua nilai statis mudah diganti dengan response API asli.
class RideMockSimulator {
  const RideMockSimulator._();

  // Tick → fase status. Satu tick = satu kali polling (3 detik), jadi
  // simulasi lengkap ± 42 detik sebelum berstatus COMPLETED.
  static const int searchingEndTick = 1;
  static const int assignedEndTick = 5;
  static const int arrivedEndTick = 7;
  static const int tripEndTick = 13;

  static const List<DriverInfo> drivers = [
    DriverInfo(
      id: '00000000-0000-0000-0000-0000000000d1',
      name: 'Ahmad Dedi',
      phone: '081234567801',
      vehicleType: 'Toyota Avanza',
      vehiclePlate: 'B 1234 XYZ',
      rating: 4.8,
      reviewCount: 342,
    ),
    DriverInfo(
      id: '00000000-0000-0000-0000-0000000000d2',
      name: 'Budi Santoso',
      phone: '081234567802',
      vehicleType: 'Honda Brio',
      vehiclePlate: 'B 9090 KLM',
      rating: 4.6,
      reviewCount: 218,
    ),
    DriverInfo(
      id: '00000000-0000-0000-0000-0000000000d3',
      name: 'Citra Lestari',
      phone: '081234567803',
      vehicleType: 'Suzuki Ertiga',
      vehiclePlate: 'B 5555 QRS',
      rating: 4.9,
      reviewCount: 501,
    ),
  ];

  static String statusForTick(int tick) {
    if (tick <= searchingEndTick) return 'SEARCHING_DRIVER';
    if (tick <= assignedEndTick) return 'DRIVER_ASSIGNED';
    if (tick <= arrivedEndTick) return 'DRIVER_ARRIVED';
    if (tick <= tripEndTick) return 'TRIP_STARTED';
    return 'COMPLETED';
  }

  static DriverInfo driverForTick(int tick) => drivers[tick ~/ 4 % drivers.length];

  /// Posisi driver simulasi: bergerak dari titik awal di utara pickup menuju
  /// pickup (DRIVER_ASSIGNED), diam di pickup (DRIVER_ARRIVED), lalu pickup →
  /// dropoff (TRIP_STARTED sampai COMPLETED).
  ///
  /// TODO: ganti dengan lokasi driver real saat backend menyediakan
  /// endpoint lokasi driver (misal GET /drivers/{id}/location).
  static LatLng driverLocationForTick({
    required LatLng pickup,
    required LatLng dropoff,
    required int tick,
  }) {
    if (tick > arrivedEndTick) {
      final progress = ((tick - arrivedEndTick) / (tripEndTick - arrivedEndTick))
          .clamp(0.0, 1.0);
      return _lerp(pickup, dropoff, progress);
    }
    if (tick > searchingEndTick) {
      final start = LatLng(pickup.latitude + 0.018, pickup.longitude - 0.009);
      final progress = ((tick - searchingEndTick) / (assignedEndTick - searchingEndTick))
          .clamp(0.0, 1.0);
      return _lerp(start, pickup, progress);
    }
    return pickup;
  }

  /// Simulasi POST /rides/book (dipakai saat backend tidak terjangkau).
  static BookRideResult book({
    required double pickupLat,
    required double pickupLng,
    required double dropoffLat,
    required double dropoffLng,
    required String paymentMethod,
  }) {
    final distance = distanceKm(
      LatLng(pickupLat, pickupLng),
      LatLng(dropoffLat, dropoffLng),
    );
    return BookRideResult(
      orderId: const Uuid().v4(),
      status: 'SEARCHING_DRIVER',
      distanceKm: distance,
      estimatedFare: (10000 + distance * 4000).round(),
      paymentMethod: paymentMethod,
      isMock: true,
    );
  }

  /// Haversine distance dalam kilometer (base_fare Rp 10.000, per_km Rp 4.000).
  static double distanceKm(LatLng a, LatLng b) {
    const earthRadiusKm = 6371.0;
    final dLat = _rad(b.latitude - a.latitude);
    final dLng = _rad(b.longitude - a.longitude);
    final h = math.pow(math.sin(dLat / 2), 2) +
        math.cos(_rad(a.latitude)) * math.cos(_rad(b.latitude)) * math.pow(math.sin(dLng / 2), 2);
    return 2 * earthRadiusKm * math.asin(math.sqrt(h));
  }

  static LatLng _lerp(LatLng a, LatLng b, double t) {
    return LatLng(
      a.latitude + (b.latitude - a.latitude) * t,
      a.longitude + (b.longitude - a.longitude) * t,
    );
  }

  static double _rad(double deg) => deg * math.pi / 180;
}