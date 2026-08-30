import '../models/menu.dart';
import '../models/menu_item.dart';
import '../models/merchant.dart';
import 'api_client.dart';

/// MerchantService — akses merchant & menu (API_CONTRACT 8.1 / 8.2).
class MerchantService {
  const MerchantService(this.apiClient);

  final ApiClient apiClient;

  /// GET /api/v1/merchants?lat=&lng=&search=&sort=
  Future<List<Merchant>> fetchMerchants({
    double? lat,
    double? lng,
    String? search,
    String sort = 'distance',
  }) async {
    final params = <String, String>{
      if (lat != null) 'lat': '$lat',
      if (lng != null) 'lng': '$lng',
      'sort': sort,
    };
    if (search != null && search.trim().isNotEmpty) params['search'] = search.trim();
    final res = await apiClient.get('/api/v1/merchants$_params(params)');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    final items = _extractList(data, 'merchants');
    return items.map(Merchant.fromJson).toList();
  }

  static String _params(Map<String, String> params) {
    if (params.isEmpty) return '';
    return '?${params.entries.map((e) => '${e.key}=${Uri.encodeQueryComponent(e.value)}').join('&')}';
  }

  /// GET /api/v1/merchants/{merchant_id}/menus — struktur menu + item.
  Future<List<Menu>> fetchMenus(String merchantId) async {
    final res = await apiClient.get('/api/v1/merchants/$merchantId/menus');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    final items = _extractList(data, 'menus');
    if (items.isNotEmpty) {
      return items.map((m) => Menu.fromJson(m, merchantId: merchantId)).toList();
    }
    return const [];
  }

  /// GET /api/v1/merchants/{merchant_id}/items — item + opsi.
  Future<List<Menu>> fetchItemsAsMenus(String merchantId) async {
    final res = await apiClient.get('/api/v1/merchants/$merchantId/items');
    final data = res.data is Map<String, dynamic> ? (res.data as Map)['data'] : null;
    return _itemsToMenus(_extractList(data, 'items'), merchantId);
  }

  static List<Map<String, dynamic>> _extractList(Object? data, String key) {
    if (data is Map && data[key] is List) {
      return (data[key] as List).whereType<Map>().cast<Map<String, dynamic>>().toList();
    }
    if (data is List) return data.whereType<Map>().cast<Map<String, dynamic>>().toList();
    return const [];
  }

  static List<Menu> _itemsToMenus(List<Map<String, dynamic>> items, String merchantId) {
    // Kelompokkan item per menu_id → Menu tiruan.
    final grouped = <String, List<Map<String, dynamic>>>{};
    for (final item in items) {
      final menuId = item['menu_id']?.toString() ?? 'others';
      grouped.putIfAbsent(menuId, () => []).add(item);
    }
    return grouped.entries.map((e) {
      final first = e.value.first;
      return Menu(
        id: e.key,
        merchantId: merchantId,
        name: first['menu_name']?.toString() ?? 'Menu',
        items: e.value
            .map((i) => MenuItem.fromJson(i, merchantId: merchantId))
            .toList(),
      );
    }).toList();
  }
}
