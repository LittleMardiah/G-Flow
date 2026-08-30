import '../models/merchant_item.dart';
import '../models/merchant_menu.dart';
import 'api_client.dart';

class MenuService {
  const MenuService(this.apiClient);

  final ApiClient apiClient;

  Future<List<MerchantMenu>> fetchMenus(String merchantId) async {
    final res = await apiClient.get('/api/v1/merchants/$merchantId/menus');
    final raw = _listOf(res.data, 'menus');
    return raw.map(MerchantMenu.fromJson).toList();
  }

  Future<List<MerchantItem>> fetchItems(String merchantId) async {
    final res = await apiClient.get('/api/v1/merchants/$merchantId/items');
    final raw = _listOf(res.data, 'items');
    return raw.map(MerchantItem.fromJson).toList();
  }

  Future<MerchantMenu> createMenu(
    String merchantId, {
    required String name,
    String? description,
    int sequenceOrder = 0,
  }) async {
    final res = await apiClient.post('/api/v1/merchants/$merchantId/menus', data: {
      'name': name,
      'description': ?description,
      'sequence_order': sequenceOrder,
    });
    return MerchantMenu.fromJson(_dataOf(res.data));
  }

  Future<MerchantMenu> updateMenu(
    String merchantId,
    String menuId, {
    String? name,
    String? description,
    bool? isActive,
  }) async {
    final res = await apiClient.patch('/api/v1/merchants/$merchantId/menus/$menuId', data: {
      'name': ?name,
      'description': ?description,
      'is_active': ?isActive,
    });
    return MerchantMenu.fromJson(_dataOf(res.data));
  }

  Future<void> deleteMenu(String merchantId, String menuId) async {
    await apiClient.delete('/api/v1/merchants/$merchantId/menus/$menuId');
  }

  Future<MerchantItem> createItem(
    String merchantId, {
    required String menuId,
    required String name,
    String? description,
    int price = 0,
    String? imageUrl,
    int stock = 999,
    bool isAvailable = true,
  }) async {
    final res = await apiClient.post('/api/v1/merchants/$merchantId/items', data: {
      'menu_id': menuId,
      'name': name,
      'description': ?description,
      'price': price,
      'image_url': ?imageUrl,
      'stock': stock,
      'is_available': isAvailable,
    });
    return MerchantItem.fromJson(_dataOf(res.data));
  }

  Future<MerchantItem> updateItem(
    String merchantId,
    String itemId, {
    String? name,
    String? description,
    int? price,
    String? imageUrl,
    int? stock,
    bool? isAvailable,
  }) async {
    final res = await apiClient.patch('/api/v1/merchants/$merchantId/items/$itemId', data: {
      'name': ?name,
      'description': ?description,
      'price': ?price,
      'image_url': ?imageUrl,
      'stock': ?stock,
      'is_available': ?isAvailable,
    });
    return MerchantItem.fromJson(_dataOf(res.data));
  }

  Future<void> deleteItem(String merchantId, String itemId) async {
    await apiClient.delete('/api/v1/merchants/$merchantId/items/$itemId');
  }

  static List<Map<String, dynamic>> _listOf(Object? raw, String key) {
    final data = raw is Map ? raw['data'] : raw;
    final list = data is Map && data[key] is List
        ? (data[key] as List)
        : (data is List ? data : const <dynamic>[]);
    return list.whereType<Map>().cast<Map<String, dynamic>>().toList();
  }

  static Map<String, dynamic> _dataOf(Object? raw) {
    if (raw is Map) {
      final data = raw['data'];
      if (data is Map<String, dynamic>) return data;
    }
    return const {};
  }
}