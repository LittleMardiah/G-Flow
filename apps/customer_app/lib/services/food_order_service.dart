import 'package:uuid/uuid.dart';

import '../models/food_order.dart';
import 'api_client.dart';

/// Flag sementara untuk tracking food order.
///
/// GET /food-orders/{id} (API_CONTRACT 8.4) sudah ada di backend, namun
/// detail driver & lokasi driver untuk tracking belum tersedia sebagai
/// field terpisah. Selama itu, screen tracking memakai FoodOrderMockSimulator
/// untuk demo latar status merchant→driver.
const bool kUseMockFoodTracking = true;

class FoodOrderService {
  const FoodOrderService(this.apiClient);

  final ApiClient apiClient;

  /// POST /api/v1/food-orders — buat food order.
  Future<FoodOrder> createOrder({
    required String merchantId,
    required List<Map<String, dynamic>> items,
    required String deliveryAddress,
    double? deliveryLat,
    double? deliveryLng,
    required String paymentMethod,
  }) async {
    final res = await apiClient.post(
      '/api/v1/food-orders',
      headers: {'X-Idempotency-Key': const Uuid().v4()},
      data: {
        'merchant_id': merchantId,
        'items': items,
        'delivery_address': deliveryAddress,
        'delivery_lat': ?deliveryLat,
        'delivery_lng': ?deliveryLng,
        'payment_method': paymentMethod,
      },
    );
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    return FoodOrder.fromJson(data is Map<String, dynamic> ? data : const {});
  }

  /// GET /api/v1/food-orders/{order_id}
  Future<FoodOrder> getOrder(String orderId) async {
    final res = await apiClient.get('/api/v1/food-orders/$orderId');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Respons tidak valid untuk food order $orderId');
    }
    return FoodOrder.fromJson(data);
  }

  /// PATCH /api/v1/food-orders/{order_id} — status (mis. CANCELLED).
  Future<void> updateStatus(String orderId, String status) async {
    await apiClient.patch(
      '/api/v1/food-orders/$orderId',
      data: {'status': status},
    );
  }

  /// GET /api/v1/food-orders?page=&limit= — riwayat order.
  Future<List<FoodOrder>> getHistory({int page = 1, int limit = 20}) async {
    final res = await apiClient.get('/api/v1/food-orders?page=$page&limit=$limit');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    final items = data is Map && data['orders'] is List
        ? data['orders'] as List
        : (data is List ? data : const <dynamic>[]);
    return items
        .whereType<Map>()
        .map((e) => FoodOrder.fromJson(e.cast<String, dynamic>()))
        .toList();
  }
}
