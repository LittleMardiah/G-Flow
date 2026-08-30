/// Item di dalam sebuah food order (API_CONTRACT 8.3).
class FoodOrderItem {
  const FoodOrderItem({
    this.orderItemId,
    required this.itemName,
    this.quantity = 1,
    this.price = 0,
    this.subtotal = 0,
    this.selectedOptions = const [],
  });

  final String? orderItemId;
  final String itemName;
  final int quantity;
  final int price;
  final int subtotal;
  final List<String> selectedOptions;

  factory FoodOrderItem.fromJson(Map<String, dynamic> j) {
    return FoodOrderItem(
      orderItemId: _str(j['order_item_id']),
      itemName: _str(j['item_name'] ?? j['name']) ?? '',
      quantity: _int(j['quantity']),
      price: _int(j['unit_price'] ?? j['price']),
      subtotal: _int(j['subtotal']),
      selectedOptions: _splitOptions(j['selected_options']),
    );
  }

  static List<String> _splitOptions(Object? v) {
    if (v is List) return v.map((e) => e.toString()).toList();
    final s = _str(v);
    if (s == null || s.isEmpty) return const [];
    return s.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}
