package com.redis.smartcache.cli.components;

import java.util.List;

import org.springframework.shell.component.context.ComponentContext;
import org.springframework.shell.component.support.Itemable;
import org.springframework.shell.component.support.Matchable;
import org.springframework.shell.component.support.Nameable;

import com.redis.smartcache.cli.components.AbstractTableSelectorComponent.ItemState;

/**
 * Context interface on a selector component sharing content.
 */
public interface SelectorComponentContext<T, I extends Nameable & Matchable & Itemable<T>, C extends SelectorComponentContext<T, I, C>>
        extends ComponentContext<C> {

    /**
     * Sets a name
     *
     * @param name the name
     */
    void setName(String name);

    /**
     * Gets an input.
     *
     * @return an input
     */
    String getInput();

    /**
     * Gets an item states
     *
     * @return an item states
     */
    List<ItemState<I>> getItemStates();

    /**
     * Sets an item states.
     *
     * @param itemStateView the input state
     */
    void setItemStates(List<ItemState<I>> itemStateView);

    /**
     * Sets an item state view
     *
     * @param itemStateView the item state view
     */
    void setItemStateView(List<ItemState<I>> itemStateView);

    /**
     * Gets a cursor row.
     *
     * @return a cursor row.
     */
    Integer getCursorRow();

    /**
     * Sets a cursor row.
     *
     * @param cursorRow the cursor row
     */
    void setCursorRow(Integer cursorRow);

    /**
     * Gets an items.
     *
     * @return an items
     */
    List<I> getItems();

    /**
     * Sets an items.
     *
     * @param items the items
     */
    void setItems(List<I> items);

    /**
     * Gets a result items.
     *
     * @return a result items
     */
    List<I> getResultItems();

    /**
     * Sets a result items.
     *
     * @param items the result items
     */
    void setResultItems(List<I> items);

}
