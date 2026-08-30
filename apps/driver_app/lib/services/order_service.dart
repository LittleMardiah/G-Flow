import 'package:dio/dio.dart';

import '../models/driver_order.dart';
import 'api_client.dart';

/// Fallback DEMO untuk fetching order bila endpoint listing belum ada di
/// backend (GET /rides/available, /food-orders/available,
/// /send-orders/available, /drivers/{id}/available-orders & GET /rides/{id}
/// belum diimplementasikan — catatan BLUEPRINT). Dataset mock ber-label
/// isMock=true dan UI menampilkan badge DEMO (konvensi customer_app).
const bool kUseMockAvailableOrders = true;

class OrderService {
  const OrderService(this.apiClient);

  final ApiClient apiClient;

  /// GET /rides/available — order ride yang sedang SEARCHING_DRIVER.
  Future<List<DriverOrder>> fetchAvailableRides() async {
    if (kUseMockAvailableOrders) return _mockRideOrders();
    final res = await apiClient.get('/api/v1/rides/available');
    return unwrapList(res.data)
        .whereType<Map>()
        .map((e) => DriverOrder.fromRideJson(e.cast<String, dynamic>(), isMock: true))
        .toList();
  }

  /// GET /food-orders/available — order food siap diambil driver.
  Future<List<DriverOrder>> fetchAvailableFoodOrders() async {
    if (kUseMockAvailableOrders) return _mockFoodOrders();
    final res = await apiClient.get('/api/v1/food-orders/available');
    return unwrapList(res.data)
        .whereType<Map>()
        .map((e) => DriverOrder.fromFoodJson(e.cast<String, dynamic>(), isMock: true))
        .toList();
  }

  /// GET /send-orders/available — order send yang SEARCHING_DRIVER.
  Future<List<DriverOrder>> fetchAvailableSendOrders() async {
    if (kUseMockAvailableOrders) return _mockSendOrders();
    final res = await apiClient.get('/api/v1/send-orders/available');
    return unwrapList(res.data)
        .whereType<Map>()
        .map((e) => DriverOrder.fromSendJson(e.cast<String, dynamic>(), isMock: true))
        .toList();
  }

  /// Semua order available: ride + food + send (dashboard & list screen).
  Future<List<DriverOrder>> fetchAvailableOrders() async {
    final results = await Future.wait([
      fetchAvailableRides(),
      fetchAvailableFoodOrders(),
      fetchAvailableSendOrders(),
    ]);
    return results.expand((e) => e).toList()
      ..sort((a, b) => a.distanceKm.compareTo(b.distanceKm));
  }

  /// GET /drivers/{driver_id}/available-orders — order aktif milik driver
  /// (mengembalikan order assigned + in-progress). Endpoint ini juga belum
  /// ada di backend → fallback mock.
  Future<List<DriverOrder>> fetchActiveOrders(String driverId) async {
    if (kUseMockAvailableOrders) return _mockActiveOrders(driverId);
    final res = await apiClient.get('/api/v1/drivers/$driverId/available-orders');
    return unwrapList(res.data)
        .whereType<Map>()
        .map((e) => DriverOrder.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  /// GET /rides/{order_id} — detail ride. Backend belum punya route GET ini;
  /// fallback ke mock agar screen detail tetap bisa ditampilkan.
  Future<DriverOrder> fetchRideDetail(String orderId) async {
    try {
      final res = await apiClient.get('/api/v1/rides/$orderId');
      final data = unwrapData(res.data);
      final order = data?['order'];
      if (order is Map<String, dynamic>) {
        return DriverOrder.fromRideJson(order);
      }
      return DriverOrder.fromRideJson({
        'order_id': orderId,
        'status': 'DRIVER_ASSIGNED',
      }, isMock: true);
    } on DioException {
      return _mockRideOrders().firstWhere(
        (o) => o.id == orderId,
        orElse: () => DriverOrder.fromRideJson(
          {'order_id': orderId, 'status': 'DRIVER_ASSIGNED'},
          isMock: true,
        ),
      );
    }
  }

  /// GET /food-orders/{order_id} — detail food (real backend).
  Future<DriverOrder> fetchFoodDetail(String orderId) async {
    final res = await apiClient.get('/api/v1/food-orders/$orderId');
    final data = unwrapData(res.data);
    final order = data?['order'] ?? data;
    return DriverOrder.fromFoodJson(
      order is Map<String, dynamic> ? order : const <String, dynamic>{},
    );
  }

  /// GET /send-orders/{order_id} — detail send + stops (real backend).
  Future<DriverOrder> fetchSendDetail(String orderId) async {
    final res = await apiClient.get('/api/v1/send-orders/$orderId');
    final data = unwrapData(res.data);
    final order = data?['order'];
    final stops = data?['stops'];
    final merged = <String, dynamic>{
      if (order is Map) ...order.cast<String, dynamic>(),
      if (stops is List) 'stops': stops,
    };
    return DriverOrder.fromSendJson(merged);
  }

  /// POST /rides/{order_id}/accept — accept order ride (real backend).
  Future<Map<String, dynamic>> acceptRide(String orderId) async {
    final res = await apiClient.post('/api/v1/rides/$orderId/accept');
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  /// POST /send-orders/{order_id}/accept — accept order send (real backend,
  /// capacity check >=3 ditolak 422/CONFLICT di server).
  Future<Map<String, dynamic>> acceptSend(String orderId) async {
    final res = await apiClient.post('/api/v1/send-orders/$orderId/accept');
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  /// "Accept" order food. Backend food belum punya endpoint accept driver;
  /// untuk MVP, accept ditandai lokal (isMock=true) lalu driver menuju
  /// merchant dan memakai PATCH status READY_FOR_PICKUP → PICKED_UP.
  Future<bool> acceptFood(String orderId) async {
    await Future<void>.delayed(const Duration(milliseconds: 300));
    return true;
  }

  /// PATCH /rides/{order_id}/status — DRIVER_ARRIVED → TRIP_STARTED → COMPLETED.
  Future<Map<String, dynamic>> updateRideStatus(String orderId, String status) async {
    final res = await apiClient.patch(
      '/api/v1/rides/$orderId/status',
      data: {'status': status},
    );
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  /// PATCH /food-orders/{order_id} — PICKED_UP → IN_TRANSIT → DELIVERED.
  Future<Map<String, dynamic>> updateFoodStatus(String orderId, String status) async {
    final res = await apiClient.patch(
      '/api/v1/food-orders/$orderId',
      data: {'status': status},
    );
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  /// PATCH /send-orders/{order_id} — PICKED_UP → IN_TRANSIT → DELIVERED.
  Future<Map<String, dynamic>> updateSendStatus(String orderId, String status) async {
    final res = await apiClient.patch(
      '/api/v1/send-orders/$orderId',
      data: {'status': status},
    );
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  /// PATCH /send-orders/{order_id}/stops/{stop_id} — mark stop COMPLETED.
  Future<Map<String, dynamic>> updateSendStop(
    String orderId,
    String stopId, {
    String status = 'COMPLETED',
    String? deliveryPhotoUrl,
  }) async {
    final res = await apiClient.patch(
      '/api/v1/send-orders/$orderId/stops/$stopId',
      data: {
        'status': status,
        'delivery_photo_url': ?deliveryPhotoUrl,
      },
    );
    return unwrapData(res.data) ?? const <String, dynamic>{};
  }

  // ---------------------------------------------------------------------------
  // Mock dataset (DEMO) — konsisten dengan koordinat kampus UI G-Flow.
  // ---------------------------------------------------------------------------

  static const double _baseLat = -6.2088;
  static const double _baseLng = 106.8456;

  List<DriverOrder> _mockRideOrders() {
    return [
      DriverOrder.fromRideJson({
        'order_id': 'mock-ride-2351',
        'status': 'SEARCHING_DRIVER',
        'pickup_address': 'Jl. Sudirman Kav 52',
        'pickup_lat': _baseLat + 0.002,
        'pickup_lng': _baseLng + 0.003,
        'dropoff_address': 'Mall Taman Anggrek',
        'dropoff_lat': _baseLat - 0.015,
        'dropoff_lng': _baseLng + 0.012,
        'distance_km': 2.5,
        'estimated_fare': 75000,
        'payment_method': 'WALLET',
        'created_at': DateTime.now().toIso8601String(),
      }, isMock: true),
      DriverOrder.fromRideJson({
        'order_id': 'mock-ride-2354',
        'status': 'SEARCHING_DRIVER',
        'pickup_address': 'Jl. Gatot Subroto',
        'pickup_lat': _baseLat - 0.004,
        'pickup_lng': _baseLng + 0.001,
        'dropoff_address': 'Kota Kasablanka',
        'dropoff_lat': _baseLat - 0.02,
        'dropoff_lng': _baseLng + 0.03,
        'distance_km': 4.1,
        'estimated_fare': 98000,
        'payment_method': 'CASH',
        'created_at': DateTime.now().toIso8601String(),
      }, isMock: true),
    ];
  }

  List<DriverOrder> _mockFoodOrders() {
    return [
      DriverOrder.fromFoodJson({
        'order_id': 'mock-food-2350',
        'merchant_name': 'Warung Mak Siti',
        'merchant_address': 'Jl. Sabang No 12',
        'merchant_lat': _baseLat + 0.004,
        'merchant_lng': _baseLng - 0.002,
        'delivery_address': 'Jl. Gatot Subroto 77',
        'delivery_lat': _baseLat - 0.008,
        'delivery_lng': _baseLng + 0.006,
        'distance_km': 1.2,
        'delivery_fee': 8000,
        'total_amount': 68000,
        'payment_method': 'WALLET',
        'status': 'READY_FOR_PICKUP',
        'created_at': DateTime.now().toIso8601String(),
      }, isMock: true),
      DriverOrder.fromFoodJson({
        'order_id': 'mock-food-2356',
        'merchant_name': 'Sate Khas Senayan',
        'merchant_address': 'Jl. Senayan 3',
        'merchant_lat': _baseLat + 0.001,
        'merchant_lng': _baseLng + 0.005,
        'delivery_address': 'Jl. Rasuna Said',
        'delivery_lat': _baseLat - 0.012,
        'delivery_lng': _baseLng + 0.009,
        'distance_km': 2.0,
        'delivery_fee': 10000,
        'total_amount': 145000,
        'payment_method': 'CASH',
        'status': 'READY_FOR_PICKUP',
        'created_at': DateTime.now().toIso8601String(),
      }, isMock: true),
    ];
  }

  List<DriverOrder> _mockSendOrders() {
    return [
      DriverOrder.fromSendJson({
        'order_id': 'mock-send-2360',
        'status': 'SEARCHING_DRIVER',
        'pickup_address': 'Jl. MH Thamrin 9',
        'pickup_lat': _baseLat + 0.006,
        'pickup_lng': _baseLng + 0.004,
        'total_distance_km': 6.3,
        'total_fare': 45000,
        'payment_method': 'WALLET',
        'created_at': DateTime.now().toIso8601String(),
        'stops': [
          {
            'stop_id': 'mock-stop-2360-1',
            'stop_number': 1,
            'recipient_name': 'Budi',
            'recipient_phone': '+628111111',
            'delivery_address': 'Jl. Prof. Dr. Satrio',
            'delivery_lat': _baseLat - 0.005,
            'delivery_lng': _baseLng + 0.008,
            'distance_km': 2.1,
            'allocated_fare': 15000,
            'status': 'PENDING',
          },
          {
            'stop_id': 'mock-stop-2360-2',
            'stop_number': 2,
            'recipient_name': 'Sari',
            'recipient_phone': '+628222222',
            'delivery_address': 'Jl. Kemang Raya',
            'delivery_lat': _baseLat - 0.011,
            'delivery_lng': _baseLng + 0.014,
            'distance_km': 2.9,
            'allocated_fare': 19000,
            'status': 'PENDING',
          },
          {
            'stop_id': 'mock-stop-2360-3',
            'stop_number': 3,
            'recipient_name': 'Ayu',
            'recipient_phone': '+628333333',
            'delivery_address': 'Jl. Fatmawati',
            'delivery_lat': _baseLat - 0.018,
            'delivery_lng': _baseLng + 0.005,
            'distance_km': 1.3,
            'allocated_fare': 11000,
            'status': 'PENDING',
          },
        ],
      }, isMock: true),
    ];
  }

  List<DriverOrder> _mockActiveOrders(String driverId) {
    return [
      DriverOrder.fromSendJson({
        'order_id': 'mock-active-send-2301',
        'status': 'IN_TRANSIT',
        'driver_id': driverId,
        'pickup_address': 'Jl. MH Thamrin 9',
        'pickup_lat': _baseLat + 0.006,
        'pickup_lng': _baseLng + 0.004,
        'total_distance_km': 6.3,
        'total_fare': 45000,
        'driver_earning': 36000,
        'payment_method': 'WALLET',
        'stops': [
          {
            'stop_id': 'mock-as-2301-1',
            'stop_number': 1,
            'recipient_name': 'Budi',
            'delivery_address': 'Jl. Prof. Dr. Satrio',
            'delivery_lat': _baseLat - 0.005,
            'delivery_lng': _baseLng + 0.008,
            'distance_km': 2.1,
            'allocated_fare': 15000,
            'status': 'COMPLETED',
          },
          {
            'stop_id': 'mock-as-2301-2',
            'stop_number': 2,
            'recipient_name': 'Sari',
            'delivery_address': 'Jl. Kemang Raya',
            'delivery_lat': _baseLat - 0.011,
            'delivery_lng': _baseLng + 0.014,
            'distance_km': 2.9,
            'allocated_fare': 19000,
            'status': 'PENDING',
          },
          {
            'stop_id': 'mock-as-2301-3',
            'stop_number': 3,
            'recipient_name': 'Ayu',
            'delivery_address': 'Jl. Fatmawati',
            'delivery_lat': _baseLat - 0.018,
            'delivery_lng': _baseLng + 0.005,
            'distance_km': 1.3,
            'allocated_fare': 11000,
            'status': 'PENDING',
          },
        ],
      }, isMock: true),
      DriverOrder.fromRideJson({
        'order_id': 'mock-active-ride-2302',
        'status': 'TRIP_STARTED',
        'driver_id': driverId,
        'pickup_address': 'Jl. Sudirman',
        'pickup_lat': _baseLat + 0.002,
        'pickup_lng': _baseLng + 0.003,
        'dropoff_address': 'Mall Taman Anggrek',
        'dropoff_lat': _baseLat - 0.015,
        'dropoff_lng': _baseLng + 0.012,
        'distance_km': 2.5,
        'estimated_fare': 75000,
        'driver_earning': 60000,
        'payment_method': 'WALLET',
      }, isMock: true),
      ];
  }
}