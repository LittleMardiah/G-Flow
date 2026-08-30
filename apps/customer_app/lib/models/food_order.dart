import 'food_order_item.dart';

/// Urutan status food order untuk timeline (API_CONTRACT 8.3 state machine).
const List<String> kFoodStatusFlow = [
  'CREATED',
  'CONFIRMED',
  'PREPARING',
  'READY_FOR_PICKUP',
  'PICKED_UP',
  'IN_TRANSIT',
  'DELIVERED',
  'SETTLED',
];

/// Status terminal food order (polling dihentikan, cancel disembunyikan).
const Set<String> kTerminalFoodStatuses = {'DELIVERED', 'SETTLED', 'CANCELLED'};

/// Food order (API_CONTRACT 8.3 / 8.4 / 8.5).
class FoodOrder {
  const FoodOrder({
    required this.id,
    this.merchantId,
    this.customerId,
    required this.status,
    this.items = const [],
    this.subtotalAmount = 0,
    this.deliveryFee = 0,
    this.discountAmount = 0,
    this.totalAmount = 0,
    this.paymentMethod = 'WALLET',
    this.deliveryAddress = '',
    this.createdAt,
    this.statusUpdatedAt,
    this.estimatedDelivery,
    this.isMock = false,
  });

  final String id;
  final String? merchantId;
  final String? customerId;
  final String status;
  final List<FoodOrderItem> items;
  final int subtotalAmount;
  final int deliveryFee;
  final int discountAmount;
  final int totalAmount;
  final String paymentMethod;
  final String deliveryAddress;
  final DateTime? createdAt;
  final DateTime? statusUpdatedAt;
  final DateTime? estimatedDelivery;
  final bool isMock;

  factory FoodOrder.fromJson(Map<String, dynamic> j, {bool isMock = false}) {
    return FoodOrder(
      id: _str(j['order_id']) ?? _str(j['id']) ?? '',
      merchantId: _str(j['merchant_id']),
      customerId: _str(j['customer_id']),
      status: _str(j['status']) ?? 'CREATED',
      items: _list(j['items']).map(FoodOrderItem.fromJson).toList(),
      subtotalAmount: _int(j['subtotal_amount']),
      deliveryFee: _int(j['delivery_fee']),
      discountAmount: _int(j['discount_amount']),
      totalAmount: _int(j['total_amount']),
      paymentMethod: _str(j['payment_method']) ?? 'WALLET',
      deliveryAddress: _str(j['delivery_address']) ?? '',
      createdAt: _date(j['created_at']),
      statusUpdatedAt: _date(j['status_updated_at']),
      estimatedDelivery: _date(j['estimated_delivery']),
      isMock: isMock,
    );
  }

  bool get isTerminal => kTerminalFoodStatuses.contains(status);
  int get currentStepIndex => kFoodStatusFlow.indexOf(status);

  static List<Map<String, dynamic>> _list(Object? v) {
    if (v is List) return v.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
  static DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v) : null;
}

/// Argumen navigasi ke /food-tracking. Digunakan sebagai fallback saat
/// GET /food-orders/{id} backend belum mengembalikan detail lengkap.
class FoodOrderTrackingArgs {
  const FoodOrderTrackingArgs({
    required this.orderId,
    this.merchantId,
    this.merchantName = '',
    this.totalAmount = 0,
    this.paymentMethod = 'WALLET',
    this.isMock = false,
  });

  final String orderId;
  final String? merchantId;
  final String merchantName;
  final int totalAmount;
  final String paymentMethod;
  final bool isMock;
}