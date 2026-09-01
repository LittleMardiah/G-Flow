import 'package:uuid/uuid.dart';

import '../models/send_order.dart';
import 'api_client.dart';

/// Flag sementara untuk tracking send order (sama dengan food: driver detail
/// belum tersedia sebagai field terpisah di backend).
const bool kUseMockSendTracking = false;

class SendOrderService {
  const SendOrderService(this.apiClient);

  final ApiClient apiClient;

  /// POST /api/v1/send-orders — buat send order (multi-stop).
  ///
  /// Body mengikuti internal/send/handler.go: pickup_address (wajib),
  /// package_weight_kg, package_type, declared_value, payment_method &
  /// stops[].recipient_name/recipient_phone/delivery_address/
  /// delivery_lat/delivery_lng. Jarak & fare dihitung backend (haversine).
  Future<SendOrder> createOrder({
    required String packageType,
    required List<Map<String, dynamic>> stops,
    required String paymentMethod,
    String pickupAddress = '',
    double pickupLat = 0,
    double pickupLng = 0,
    double weightKg = 0,
    double declaredValue = 0,
  }) async {
    final res = await apiClient.post(
      '/api/v1/send-orders',
      headers: {'X-Idempotency-Key': const Uuid().v4()},
      data: {
        'pickup_lat': pickupLat,
        'pickup_lng': pickupLng,
        'pickup_address': pickupAddress,
        'package_weight_kg': weightKg,
        'package_type': packageType,
        'declared_value': declaredValue,
        'payment_method': paymentMethod,
        'stops': stops,
      },
    );
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    return SendOrder.fromJson(data is Map<String, dynamic> ? data : const {});
  }

  /// GET /api/v1/send-orders/{order_id}
  Future<SendOrder> getOrder(String orderId) async {
    final res = await apiClient.get('/api/v1/send-orders/$orderId');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk send order $orderId');
    }
    return SendOrder.fromJson(data);
  }

  /// PATCH /api/v1/send-orders/{order_id} — status (mis. CANCELLED).
  Future<void> updateStatus(String orderId, String status) async {
    await apiClient.patch(
      '/api/v1/send-orders/$orderId',
      data: {'status': status},
    );
  }

  /// GET /api/v1/send-orders?page=&limit= — riwayat order.
  Future<List<SendOrder>> getHistory({int page = 1, int limit = 20}) async {
    final res = await apiClient.get('/api/v1/send-orders?page=$page&limit=$limit');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    final items = data is Map && data['orders'] is List
        ? data['orders'] as List
        : (data is List ? data : const <dynamic>[]);
    return items
        .whereType<Map>()
        .map((e) => SendOrder.fromJson(e.cast<String, dynamic>()))
        .toList();
  }
}
