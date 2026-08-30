import '../models/merchant_order.dart';
import 'api_client.dart';

class OrderService {
  const OrderService(this.apiClient);

  final ApiClient apiClient;

  Future<List<MerchantOrder>> fetchOrders(String merchantId, {String? status}) async {
    final q = status != null && status.isNotEmpty ? '?status=${Uri.encodeQueryComponent(status)}' : '';
    final res = await apiClient.get('/api/v1/merchants/$merchantId/orders$q');
    final raw = res.data;
    final data = raw is Map ? raw['data'] : raw;
    final list = data is Map && data['orders'] is List
        ? data['orders'] as List
        : (data is List ? data : const <dynamic>[]);
    return list
        .whereType<Map>()
        .map((e) => MerchantOrder.fromJson(e.cast<String, dynamic>()))
        .toList();
  }

  Future<MerchantOrder> fetchOrder(String orderId) async {
    final res = await apiClient.get('/api/v1/food-orders/$orderId');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) {
      throw StateError('Detail order tidak valid');
    }
    final order = data['order'];
    final items = data['items'];
    final merged = <String, dynamic>{
      if (order is Map) ...order.cast<String, dynamic>(),
      'items': items is List ? items : const <dynamic>[],
    };
    return MerchantOrder.fromJson(merged);
  }

  Future<Map<String, dynamic>> updateStatus(String orderId, String status) async {
    final res = await apiClient.patch('/api/v1/food-orders/$orderId', data: {'status': status});
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    if (data is! Map<String, dynamic>) return const {};
    return data;
  }
}