import 'package:latlong2/latlong.dart';

/// Urutan status ride yang ditampilkan pada stepper tracking
/// (ROADMAP 02 bagian 3.2 — Customer App).
const List<String> kRideStatusFlow = [
  'SEARCHING_DRIVER',
  'DRIVER_ASSIGNED',
  'DRIVER_ARRIVED',
  'TRIP_STARTED',
  'COMPLETED',
];

/// Status terminal: polling dihentikan & aksi cancel disembunyikan.
const Set<String> kTerminalRideStatuses = {'COMPLETED', 'SETTLED', 'CANCELLED'};

/// Nominal cancellation fee untuk reason CUSTOMER_CANCEL (TD-131).
///
/// HARUS sinkron dengan `CancellationFeeAssigned` / `CancellationFeeArrived`
/// di internal/ride/service.go — backend adalah source of truth penentuan
/// fee (`cancellationFeeFor`). Angka di sini hanya untuk *preview* di dialog
/// konfirmasi; nilai yang benar-benar dipungut dibaca dari field
/// `cancellation_fee` pada response PATCH /rides/:id/status.
const int kRideCancellationFeeAssigned = 5000;
const int kRideCancellationFeeArrived = 10000;

/// Preview fee pembatalan per status order (reason CUSTOMER_CANCEL).
///
/// Padanan `cancellationFeeFor` (internal/ride/service.go) untuk reason
/// CUSTOMER_CANCEL: cancel sebelum driver ditunjuk → 0; DRIVER_ASSIGNED →
/// 5.000; DRIVER_ARRIVED maupun TRIP_STARTED → 10.000.
const Map<String, int> kRideCancellationFeeByStatus = {
  'SEARCHING_DRIVER': 0,
  'DRIVER_ASSIGNED': kRideCancellationFeeAssigned,
  'DRIVER_ARRIVED': kRideCancellationFeeArrived,
  'TRIP_STARTED': kRideCancellationFeeArrived,
};

/// Fee pembatalan untuk [status] (0 bila status tidak dikenali atau tidak
/// bisa di-cancel). Status COMPLETED/SETTLED tidak bisa dibatalkan sehingga
/// selalu 0 — aksi cancel memang disembunyikan di status tersebut.
int rideCancellationFeeForStatus(String? status) =>
    status == null ? 0 : (kRideCancellationFeeByStatus[status] ?? 0);

/// Model ride order.
///
/// Disimpan lokal di customer_app (packages/core belum ada di monorepo ini).
/// Field mengikuti response GET /rides/{order_id} pada API_CONTRACT 7.4.
class RideOrder {
  const RideOrder({
    required this.id,
    this.driverId,
    this.driver,
    required this.status,
    required this.pickupLat,
    required this.pickupLng,
    required this.dropoffLat,
    required this.dropoffLng,
    this.pickupAddress = '',
    this.dropoffAddress = '',
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.baseFare = 0,
    this.perKmRate = 0,
    this.actualFare,
    this.discountAmount,
    this.paymentMethod = 'WALLET',
    this.createdAt,
    this.completedAt,
    this.settledAt,
    this.cancellationReason,
    this.cancellationFee,
  });

  final String id;
  final String? driverId;
  final DriverInfo? driver;
  final String status;
  final double pickupLat;
  final double pickupLng;
  final double dropoffLat;
  final double dropoffLng;
  final String pickupAddress;
  final String dropoffAddress;
  final double distanceKm;
  final int estimatedFare;
  final int baseFare;
  final int perKmRate;
  final int? actualFare;
  final int? discountAmount;
  final String paymentMethod;
  final DateTime? createdAt;
  final DateTime? completedAt;
  final DateTime? settledAt;
  final String? cancellationReason;

  /// Fee pembatalan yang benar-benar dipungut backend (TD-131). Null sebelum
  /// order dibatalkan atau saat response tidak menyertakan field ini
  /// (`cancellation_fee` di-omitempty di PATCH status saat nil).
  final int? cancellationFee;

  factory RideOrder.fromJson(Map<String, dynamic> j) {
    final driverJson = j['driver'];
    final driver = driverJson is Map
        ? DriverInfo.fromJson(driverJson.cast<String, dynamic>())
        : null;
    final rawDriverId = _str(j['driver_id']);
    final driverId = rawDriverId != null && rawDriverId.isNotEmpty
        ? rawDriverId
        : (driver != null && driver.id.isNotEmpty ? driver.id : null);

    return RideOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      driverId: driverId,
      driver: driver,
      status: _str(j['status']) ?? 'SEARCHING_DRIVER',
      pickupLat: _num(j['pickup_lat']),
      pickupLng: _num(j['pickup_lng']),
      dropoffLat: _num(j['dropoff_lat']),
      dropoffLng: _num(j['dropoff_lng']),
      pickupAddress: _str(j['pickup_address']) ?? '',
      dropoffAddress: _str(j['dropoff_address']) ?? '',
      distanceKm: _num(j['distance_km']),
      estimatedFare: _int(j['estimated_fare']),
      baseFare: _int(j['base_fare']),
      perKmRate: _int(j['per_km_rate']),
      actualFare: _intNullable(j['actual_fare']),
      discountAmount: _intNullable(j['discount_amount']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      createdAt: _date(j['created_at']),
      completedAt: _date(j['completed_at']),
      settledAt: _date(j['settled_at']),
      cancellationReason: _str(j['cancellation_reason']),
      cancellationFee: _intNullable(j['cancellation_fee']),
    );
  }

  RideOrder copyWith({
    String? status,
    String? cancellationReason,
    int? cancellationFee,
    DriverInfo? driver,
  }) {
    return RideOrder(
      id: id,
      driverId: driverId,
      driver: driver ?? this.driver,
      status: status ?? this.status,
      pickupLat: pickupLat,
      pickupLng: pickupLng,
      dropoffLat: dropoffLat,
      dropoffLng: dropoffLng,
      pickupAddress: pickupAddress,
      dropoffAddress: dropoffAddress,
      distanceKm: distanceKm,
      estimatedFare: estimatedFare,
      baseFare: baseFare,
      perKmRate: perKmRate,
      actualFare: actualFare,
      discountAmount: discountAmount,
      paymentMethod: paymentMethod,
      createdAt: createdAt,
      completedAt: completedAt,
      settledAt: settledAt,
      cancellationReason: cancellationReason ?? this.cancellationReason,
      cancellationFee: cancellationFee ?? this.cancellationFee,
    );
  }

  LatLng get pickup => LatLng(pickupLat, pickupLng);
  LatLng get dropoff => LatLng(dropoffLat, dropoffLng);

  bool get isTerminal => kTerminalRideStatuses.contains(status);
  int get currentStepIndex => kRideStatusFlow.indexOf(status);

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static double _num(Object? v) => v is num ? v.toDouble() : (double.tryParse('$v') ?? 0);
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static int? _intNullable(Object? v) {
    if (v is num) return v.round();
    return v == null || '$v'.isEmpty ? null : int.tryParse('$v');
  }
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Argumen navigasi ke `/ride-detail`. Diteruskan via
/// `Navigator.pushNamed(context, AppRoutes.rideDetail, arguments: ...)`.
class RideDetailArgs {
  const RideDetailArgs({required this.orderId});

  final String orderId;
}

/// Info driver yang ditampilkan di kartu driver pada tracking screen.
class DriverInfo {
  const DriverInfo({
    required this.id,
    this.name = '',
    this.phone = '',
    this.vehicleType = '',
    this.vehiclePlate = '',
    this.photoUrl,
    this.rating = 0,
    this.reviewCount = 0,
  });

  final String id;
  final String name;
  final String phone;
  final String vehicleType;
  final String vehiclePlate;
  final String? photoUrl;
  final double rating;
  final int reviewCount;

  factory DriverInfo.fromJson(Map<String, dynamic> j) {
    return DriverInfo(
      id: (j['id'] ?? j['driver_id'] ?? '').toString(),
      name: (j['name'] ?? j['full_name'] ?? '').toString(),
      phone: (j['phone_masked'] ?? j['phone'] ?? '').toString(),
      vehicleType: (j['vehicle_type'] ?? '').toString(),
      vehiclePlate: (j['vehicle_plate'] ?? '').toString(),
      photoUrl: j['photo_url']?.toString(),
      rating: (j['rating'] is num ? j['rating'] as num : 0).toDouble(),
      reviewCount: (j['review_count'] is num ? j['review_count'] as num : 0).round(),
    );
  }
}

/// Hasil POST /rides/book (API_CONTRACT 7.1). isMock=true menandakan hasil
/// berasal dari simulasi (backend endpoint tracking belum tersedia).
class BookRideResult {
  const BookRideResult({
    required this.orderId,
    required this.status,
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final String status;
  final double distanceKm;
  final int estimatedFare;
  final String paymentMethod;
  final bool isMock;

  factory BookRideResult.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    return BookRideResult(
      orderId: (j['order_id'] ?? '').toString(),
      status: (j['status'] ?? 'SEARCHING_DRIVER').toString(),
      distanceKm: (j['distance_km'] is num ? j['distance_km'] as num : 0).toDouble(),
      estimatedFare: (j['estimated_fare'] is num ? j['estimated_fare'] as num : 0).round(),
      paymentMethod: (j['payment_method'] ?? 'WALLET').toString(),
      isMock: isMock,
    );
  }
}

/// Argumen navigasi ke `/ride-tracking`. Diteruskan via
/// `Navigator.pushNamed(context, AppRoutes.rideTracking, arguments: ...)`
/// agar tracking screen tetap berfungsi walaupun GET /rides/{id} backend
/// belum mengembalikan koordinat (fallback mock menggunakan nilai ini).
class RideTrackingArgs {
  const RideTrackingArgs({
    required this.orderId,
    required this.pickupLat,
    required this.pickupLng,
    required this.dropoffLat,
    required this.dropoffLng,
    this.pickupAddress = '',
    this.dropoffAddress = '',
    this.distanceKm = 0,
    this.estimatedFare = 0,
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final double pickupLat;
  final double pickupLng;
  final double dropoffLat;
  final double dropoffLng;
  final String pickupAddress;
  final String dropoffAddress;
  final double distanceKm;
  final int estimatedFare;
  final String paymentMethod;
  final bool isMock;
}