import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/merchant_item.dart';
import '../models/merchant_menu.dart';
import 'auth_provider.dart';

class MenuState {
  final List<MerchantMenu> menus;
  final List<MerchantItem> items;
  final bool isLoading;
  final String? error;
  final bool isMutating;

  const MenuState({
    this.menus = const [],
    this.items = const [],
    this.isLoading = false,
    this.error,
    this.isMutating = false,
  });

  MenuState copyWith({
    List<MerchantMenu>? menus,
    List<MerchantItem>? items,
    bool? isLoading,
    String? error,
    bool? isMutating,
  }) {
    return MenuState(
      menus: menus ?? this.menus,
      items: items ?? this.items,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      isMutating: isMutating ?? this.isMutating,
    );
  }
}

class MenuNotifier extends StateNotifier<MenuState> {
  MenuNotifier(this.ref) : super(const MenuState());

  final Ref ref;

  Future<void> load() async {
    final merchantId = await _merchantId();
    if (merchantId == null) return;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final service = ref.read(menuServiceProvider);
      final menus = await service.fetchMenus(merchantId);
      final items = await service.fetchItems(merchantId);
      state = state.copyWith(menus: menus, items: items, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  List<MerchantItem> itemsForMenu(String menuId) {
    return state.items.where((i) => i.menuId == menuId).toList();
  }

  Future<bool> createMenu({required String name, String? description}) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).createMenu(merchantId, name: name, description: description);
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<bool> updateMenu(String menuId, {String? name, String? description, bool? isActive}) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).updateMenu(merchantId, menuId, name: name, description: description, isActive: isActive);
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<bool> deleteMenu(String menuId) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).deleteMenu(merchantId, menuId);
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<bool> createItem({
    required String menuId,
    required String name,
    String? description,
    int price = 0,
    String? imageUrl,
    int stock = 999,
    bool isAvailable = true,
  }) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).createItem(
        merchantId,
        menuId: menuId,
        name: name,
        description: description,
        price: price,
        imageUrl: imageUrl,
        stock: stock,
        isAvailable: isAvailable,
      );
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<bool> updateItem(
    String itemId, {
    String? name,
    String? description,
    int? price,
    String? imageUrl,
    int? stock,
    bool? isAvailable,
  }) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).updateItem(
        merchantId,
        itemId,
        name: name,
        description: description,
        price: price,
        imageUrl: imageUrl,
        stock: stock,
        isAvailable: isAvailable,
      );
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<bool> deleteItem(String itemId) async {
    final merchantId = await _merchantId();
    if (merchantId == null) return false;
    state = state.copyWith(isMutating: true, error: null);
    try {
      await ref.read(menuServiceProvider).deleteItem(merchantId, itemId);
      state = state.copyWith(isMutating: false);
      await load();
      return true;
    } catch (e) {
      state = state.copyWith(isMutating: false, error: e.toString());
      return false;
    }
  }

  Future<String?> _merchantId() async {
    final merchantId = await ref.read(storageProvider).read(key: kMerchantIdKey);
    if (merchantId == null || merchantId.isEmpty) {
      state = state.copyWith(error: 'ID merchant belum tersedia. Login ulang.');
      return null;
    }
    return merchantId;
  }
}

final menuProvider = StateNotifierProvider<MenuNotifier, MenuState>((ref) {
  return MenuNotifier(ref);
});