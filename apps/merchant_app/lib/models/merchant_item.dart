class MerchantItem {
  const MerchantItem({
    required this.id,
    this.merchantId,
    this.menuId,
    this.menuName,
    this.name = '',
    this.description,
    this.price = 0,
    this.imageUrl,
    this.stock = 999,
    this.isAvailable = true,
  });

  final String id;
  final String? merchantId;
  final String? menuId;
  final String? menuName;
  final String name;
  final String? description;
  final int price;
  final String? imageUrl;
  final int stock;
  final bool isAvailable;

  bool get outOfStock => stock == 0;

  MerchantItem copyWith({
    String? name,
    String? description,
    int? price,
    String? imageUrl,
    int? stock,
    bool? isAvailable,
    String? menuId,
  }) {
    return MerchantItem(
      id: id,
      merchantId: merchantId,
      menuId: menuId ?? this.menuId,
      menuName: menuName,
      name: name ?? this.name,
      description: description ?? this.description,
      price: price ?? this.price,
      imageUrl: imageUrl ?? this.imageUrl,
      stock: stock ?? this.stock,
      isAvailable: isAvailable ?? this.isAvailable,
    );
  }

  factory MerchantItem.fromJson(Map<String, dynamic> j) {
    return MerchantItem(
      id: _str(j['item_id']) ?? _str(j['id']) ?? '',
      merchantId: _str(j['merchant_id']),
      menuId: _str(j['menu_id']),
      menuName: _str(j['menu_name']),
      name: _str(j['name']) ?? '',
      description: _str(j['description']),
      price: _int(j['price']),
      imageUrl: _str(j['image_url'] ?? j['image']),
      stock: j['stock'] == null ? 999 : _int(j['stock']),
      isAvailable: j['is_available'] != false && j['available'] != false,
    );
  }

  static String? _str(Object? v) =>
      v == null ? null : v is String ? v : v.toString();
  static int _int(Object? v) => v is num ? v.round() : (int.tryParse('$v') ?? 0);
}